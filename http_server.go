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

	if addrNode, err := conf.At("address"); err == nil {
		hs.address, _ = addrNode.GetString()
	}

	if portNode, err := conf.At("port"); err == nil {
		if v, _ := portNode.GetInt(); v > 0 {
			hs.port = v
		}
	}

	if keyNode, err := conf.At("api_key"); err == nil {
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
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization", "X-API-Key"},
		AllowCredentials: false,
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

////////////////////////////////////////////////////////////////////////////////
// GET
////////////////////////////////////////////////////////////////////////////////

func (hs *http_server) onGet(w http.ResponseWriter, r *http.Request) {
	if !hs.checkAccess(r) {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	// Manager locks internally
	data := hs.buildConfigState()

	writeSuccess(w, data)
}

////////////////////////////////////////////////////////////////////////////////
// POST
////////////////////////////////////////////////////////////////////////////////

func (hs *http_server) onPost(w http.ResponseWriter, r *http.Request) {
	if !hs.checkAccess(r) {
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
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid JSON: %s", err))
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

	// Validate config hash BEFORE modification
	currentHash := HashSHA256(*(hs.manager.Source().getConfig()))
	if currentHash != configHash {
		writeError(w, http.StatusConflict, "config hash mismatch: config changed by someone else")
		return
	}

	// Execute operation (Manager locks internally)
	switch op {
	case "insert":
		index, err := getIndex(bodyJSON)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := hs.manager.insert(path, index, value); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

	case "remove":
		index, err := getIndex(bodyJSON)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := hs.manager.remove(path, index); err != nil {
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

	// Build updated config for response
	data := hs.buildConfigState()
	writeSuccess(w, data)
}

////////////////////////////////////////////////////////////////////////////////
// OPTIONS
////////////////////////////////////////////////////////////////////////////////

func (hs *http_server) onOptions(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Origin, Content-Type, X-API-Key")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	writeSuccess(w, orderedmap.New())
}

////////////////////////////////////////////////////////////////////////////////
// BUILD CONFIG STATE
////////////////////////////////////////////////////////////////////////////////

func (hs *http_server) buildConfigState() *orderedmap.OrderedMap {
	confJSON := orderedmap.New()
	schemaJSON := orderedmap.New()

	_ = json.Unmarshal([]byte(*(hs.manager.Source().getConfig())), &confJSON)
	_ = json.Unmarshal([]byte(*(hs.manager.Source().getSchema())), &schemaJSON)

	paths := orderedmap.New()
	paths.Set("insertable", hs.manager.getInsertablePaths())
	paths.Set("removable", hs.manager.getRemovablePaths())
	paths.Set("replaceable", hs.manager.getReplaceablePaths())

	out := orderedmap.New()
	out.Set("modifiable_paths", paths)
	out.Set("config", confJSON)
	out.Set("schema", schemaJSON)
	out.Set("config_hash", HashSHA256(*(hs.manager.Source().getConfig())))

	return out
}

////////////////////////////////////////////////////////////////////////////////
// HELPERS
////////////////////////////////////////////////////////////////////////////////

func HashSHA256(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

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

func getString(m *orderedmap.OrderedMap, key string) (string, error) {
	v, ok := m.Get(key)
	if !ok {
		return "", fmt.Errorf("%s missing", key)
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("%s must be string", key)
	}
	return s, nil
}

func getIndex(m *orderedmap.OrderedMap) (int, error) {
	val, ok := m.Get("index")
	if !ok {
		return 0, fmt.Errorf("index missing")
	}
	f, ok := val.(float64)
	if !ok {
		return 0, fmt.Errorf("index must be number")
	}
	return int(f), nil
}

func (hs *http_server) checkAccess(r *http.Request) bool {
	return r.Header.Get("X-API-Key") == hs.api_key
}
