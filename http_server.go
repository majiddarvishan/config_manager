package config

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/iancoleman/orderedmap"
	"github.com/rs/cors"
)

type http_server struct {
	address string
	port    int
	api_key string

	manager *Manager
}

func NewHttpServer(m *Manager, conf *Node) *http_server {
	hs := &http_server{manager: m}

	addrNode, err := conf.At("address")
	if err == nil {
		hs.address, _ = addrNode.GetString()
	}

	portNode, err := conf.At("port")
	if err == nil {
		if v, _ := portNode.GetInt(); v > 0 {
			hs.port = v
		}
	}

	keyNode, err := conf.At("api_key")
	if err == nil {
		hs.api_key, _ = keyNode.GetString()
	}

	return hs
}

func (hs *http_server) Start() {
	mux := http.NewServeMux()
	mux.HandleFunc("/config", hs.handleConfig)

	addr := fmt.Sprintf("%s:%d", hs.address, hs.port)

	handler := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization", "X-API-Key"},
		AllowCredentials: true,
	}).Handler(mux)

	if err := http.ListenAndServe(addr, handler); err != nil {
		fmt.Printf("error on listening server: %s\n", err)
		os.Exit(1)
	}
}

func (hs *http_server) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		hs.onGet(w, r)
	case http.MethodPost:
		hs.onPost(w, r)
	case http.MethodOptions:
		hs.onOptions(w)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (hs *http_server) onGet(w http.ResponseWriter, r *http.Request) {
	if !hs.check_access(r) {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	hs.manager.mu.RLock()
	data := hs.buildConfigStateUnsafe() // called while RLock held
	hs.manager.mu.RUnlock()

	writeSuccess(w, data)
}

func (hs *http_server) onPost(w http.ResponseWriter, r *http.Request) {
	if !hs.check_access(r) {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	defer r.Body.Close()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("could not read body: %s", err))
		return
	}

	var bodyJSON = orderedmap.New()
	if err := json.Unmarshal(body, &bodyJSON); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("could not parse JSON: %s", err))
		return
	}

	op, err := getString(bodyJSON, "op")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	path, err := getString(bodyJSON, "path")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	value, _ := bodyJSON.Get("value")
	configHash, err := getString(bodyJSON, "config_hash")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Acquire write lock to ensure atomic check-and-update
	hs.manager.mu.Lock()
	defer hs.manager.mu.Unlock()

	currentHash := HashSHA256(*(hs.manager.source.getConfig()))
	if configHash != currentHash {
		writeError(w, http.StatusConflict, "config hash mismatch: config was modified elsewhere")
		return
	}

	switch op {
	case "insert":
		idx, err := getIndex(bodyJSON)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := hs.manager.insert(path, idx, value); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

	case "remove":
		idx, err := getIndex(bodyJSON)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := hs.manager.remove(path, idx); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

	case "replace":
		if err := hs.manager.replace(path, value); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

	default:
		writeError(w, http.StatusBadRequest, "unsupported operation")
		return
	}

	// build and return the new state while still holding the write lock to ensure consistency
	data := hs.buildConfigStateUnsafe()
	writeSuccess(w, data)
}

func (hs *http_server) onOptions(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Origin, Content-Type, X-API-Key")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	writeSuccess(w, orderedmap.New()) // returns success: true with empty data
}

func (hs *http_server) check_access(r *http.Request) bool {
	return r.Header.Get("X-API-Key") == hs.api_key
}

func HashSHA256(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

/* ------------------ Config State Builder ------------------ */

/*
buildConfigStateUnsafe builds the response data object (not wrapped in success/error).
**IMPORTANT**: this function does not take any locks. Callers MUST hold either
a read lock (RLock) or a write lock (Lock) on hs.manager.mu before calling.
*/
func (hs *http_server) buildConfigStateUnsafe() *orderedmap.OrderedMap {
	confJSON := orderedmap.New()
	schemaJSON := orderedmap.New()

	// ignore unmarshal errors here; if underlying config/schema are invalid JSON,
	// best-effort to return whatever we can.
	_ = json.Unmarshal([]byte(*(hs.manager.source.getConfig())), &confJSON)
	_ = json.Unmarshal([]byte(*(hs.manager.source.getSchema())), &schemaJSON)

	modPaths := orderedmap.New()
	modPaths.Set("insertable", hs.manager.getInsertablePaths())
	modPaths.Set("removable", hs.manager.getRemovablePaths())
	modPaths.Set("replaceable", hs.manager.getReplaceablePaths())

	data := orderedmap.New()
	data.Set("modifiable_paths", modPaths)
	data.Set("config", confJSON)
	data.Set("schema", schemaJSON)
	data.Set("config_hash", HashSHA256(*(hs.manager.source.getConfig())))

	return data
}

/* ------------------ Unified Response Helpers ------------------ */

func writeError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)

	errObj := orderedmap.New()
	errObj.Set("message", msg)
	errObj.Set("code", code)

	resp := orderedmap.New()
	resp.Set("success", false)
	resp.Set("error", errObj)

	out, _ := json.MarshalIndent(resp, "", "  ")
	w.Write(out)
}

func writeSuccess(w http.ResponseWriter, data *orderedmap.OrderedMap) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")

	resp := orderedmap.New()
	resp.Set("success", true)
	resp.Set("data", data)

	out, _ := json.MarshalIndent(resp, "", "  ")
	w.Write(out)
}

/* ------------------ JSON Extract Helpers ------------------ */

func getString(m *orderedmap.OrderedMap, key string) (string, error) {
	val, present := m.Get(key)
	if !present {
		return "", fmt.Errorf("%s is missing", key)
	}

	s, ok := val.(string)
	if !ok {
		return "", fmt.Errorf("%s must be a string", key)
	}

	return s, nil
}

func getIndex(m *orderedmap.OrderedMap) (int, error) {
	val, present := m.Get("index")
	if !present {
		return 0, fmt.Errorf("index is missing")
	}

	f, ok := val.(float64)
	if !ok {
		return 0, fmt.Errorf("index must be a number")
	}

	return int(f), nil
}
