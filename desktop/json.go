package main

import (
	"encoding/json"
	"net/http"
)

// writeJSONBody 写 JSON 响应（桌面端点的统一出口）。
func writeJSONBody(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(body)
}

// writeJSONError 写统一错误契约：{"error": "<message>"} + 语义化状态码。
func writeJSONError(w http.ResponseWriter, status int, msg string) {
	writeJSONBody(w, status, map[string]string{"error": msg})
}
