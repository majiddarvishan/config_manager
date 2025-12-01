package config

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"

	// "net"
	"net/http"
	"os"

	"github.com/iancoleman/orderedmap"
	"github.com/rs/cors"
)

type http_server struct {
	address string
	port    int16
	api_key string

	manager *Manager
}

func NewHttpServer(m *Manager, conf *Node) *http_server {
	hs := &http_server{
		manager: m,
	}

	p, err := conf.At("address")
	if err != nil {
		fmt.Printf("Error %v is occured when reading address", err)
	}
	hs.address, err = p.GetString()
	if err != nil {
		fmt.Printf("Error %v is occured when reading address", err)
	}

	p, err = conf.At("port")
	if err != nil {
		fmt.Printf("Error %v is occured when reading port", err)
	}
	val, err := p.GetInt()
	if err != nil {
		fmt.Printf("Error %v is occured when reading port", err)
	}
	hs.port = int16(val)

	p, err = conf.At("api_key")
	if err != nil {
		fmt.Printf("Error %v is occured when reading api_key", err)
	}
	hs.api_key, err = p.GetString()
	if err != nil {
		fmt.Printf("Error %v is occured when reading api_key", err)
	}

	return hs
}

func (hs *http_server) Start() {
	sm := http.NewServeMux()

	sm.HandleFunc("/config", hs.handleConfig)

	addr := fmt.Sprintf("%s:%d", hs.address, hs.port)

	handler := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization", "X-API-Key"},
		AllowCredentials: true,
	}).Handler(sm)

	err := http.ListenAndServe(addr, handler)
	if err != nil {
		fmt.Printf("error on listening server: %s\n", err)
		os.Exit(1)
	}
}

func (hs *http_server) handleConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		hs.on_get_request(w, r)
	} else if r.Method == "POST" {
		hs.on_post_request(w, r)
	} else if r.Method == "OPTIONS" {
		hs.on_options_request(w, r)
	}
}

func (hs *http_server) on_get_request(w http.ResponseWriter, r *http.Request) {
	if !hs.check_access(r) {
		hs.unauthorized_access_resp(w)
		return
	}

	hs.latest_config_state(w)
}

func (hs *http_server) on_post_request(w http.ResponseWriter, r *http.Request) {
	if !hs.check_access(r) {
		hs.unauthorized_access_resp(w)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.Header().Set("access_control_allow_origin", "*")
		w.Header().Set("content_type", "text/html")
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, fmt.Sprintf("could not read body: %s", err))
		return
	}

	var body_json = orderedmap.New()
	if err = json.Unmarshal([]byte(body), &body_json); err != nil {
		w.Header().Set("access_control_allow_origin", "*")
		w.Header().Set("content_type", "text/html")
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, fmt.Sprintf("could not read body: %s", err))
		return
	}

	val, present := body_json.Get("op")
	if !present {
		w.Header().Set("access_control_allow_origin", "*")
		w.Header().Set("content_type", "text/html")
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, fmt.Sprintf("op tag is not present"))
		return
	}
	var op = val.(string)

	val, present = body_json.Get("path")
	if !present {
		w.Header().Set("access_control_allow_origin", "*")
		w.Header().Set("content_type", "text/html")
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, fmt.Sprintf("path tag is not present"))
		return
	}
	var path = val.(string)

	val, present = body_json.Get("value")
	if !present {
		w.Header().Set("access_control_allow_origin", "*")
		w.Header().Set("content_type", "text/html")
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, fmt.Sprintf("value tag is not present"))
		return
	}
	// var value = val.(orderedmap.OrderedMap)
	var value = val

	val, present = body_json.Get("config_hash")
	if !present {
		w.Header().Set("access_control_allow_origin", "*")
		w.Header().Set("content_type", "text/html")
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, fmt.Sprintf("config_hash tag is not present"))
		return
	}
	var config_hash = val.(string)

	// var config_hash = body_json["config_hash"].(string)
	if config_hash != HashSHA256(*(hs.manager.source.getConfig())) {
		fmt.Println("Config hash is invalid, application config is modified from else where")
		return
	}

	if op == "insert" {
		val, present = body_json.Get("index")
		if !present {
			w.Header().Set("access_control_allow_origin", "*")
			w.Header().Set("content_type", "text/html")
			w.WriteHeader(http.StatusBadRequest)
			io.WriteString(w, fmt.Sprintf("index tag is not present"))
			return
		}
		var index = int(val.(float64))

		hs.manager.insert(path, index, value)
	} else if op == "remove" {
		val, present = body_json.Get("index")
		if !present {
			w.Header().Set("access_control_allow_origin", "*")
			w.Header().Set("content_type", "text/html")
			w.WriteHeader(http.StatusBadRequest)
			io.WriteString(w, fmt.Sprintf("index tag is not present"))
			return
		}
		var index = int(val.(float64))

		hs.manager.remove(path, index)
	} else if op == "replace" {
		hs.manager.replace(path, value)
	} else {
		w.Header().Set("access_control_allow_origin", "*")
		w.Header().Set("content_type", "text/html")
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, "unsupported operation")
		return
	}

	hs.latest_config_state(w)
}

func (hs *http_server) on_options_request(w http.ResponseWriter, r *http.Request) {
	//Allow CORS here By * or specific origin
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Origin, Content-Type, X-API-Key")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, PUT, DELETE, OPTIONS")
	// w.WriteHeader(http.StatusOK)
}

func (hs *http_server) check_access(r *http.Request) bool {
	return (r.Header.Get("X-API-Key") == hs.api_key)
}

func HashSHA256(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func (hs *http_server) latest_config_state(w http.ResponseWriter) {
	var conf_json = orderedmap.New()
	json.Unmarshal([]byte(*(hs.manager.source.getConfig())), &conf_json)

	var schema_json = orderedmap.New()
	json.Unmarshal([]byte(*(hs.manager.source.getSchema())), &schema_json)

	var modifiable_paths_map = orderedmap.New()
	modifiable_paths_map.Set("insertable", hs.manager.getInsertablePaths())
	modifiable_paths_map.Set("removable", hs.manager.getRemovablePaths())
	modifiable_paths_map.Set("replaceable", hs.manager.getReplaceablePaths())

	var body_map = orderedmap.New()
	body_map.Set("modifiable_paths", modifiable_paths_map)
	body_map.Set("config", conf_json)
	body_map.Set("schema", schema_json)
	body_map.Set("config_hash", HashSHA256(*(hs.manager.source.getConfig())))

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")
	p, _ := json.MarshalIndent(body_map, "", "  ")
	io.WriteString(w, string(p))
}

func (hs *http_server) unauthorized_access_resp(w http.ResponseWriter) {
	w.Header().Set("access_control_allow_origin", "*")
	w.Header().Set("content_type", "text/html")
	w.WriteHeader(http.StatusBadRequest)
	io.WriteString(w, "Unauthorized access")

	// router::res_t res;
	// res.result(beast::http::status::unauthorized);
	// res.set(beast::http::field::access_control_allow_origin, "*");
	// res.set(beast::http::field::content_type, "text/html");
	// res.body() = "Unauthorized access";
	// res.prepare_payload();
	// return res;
}
