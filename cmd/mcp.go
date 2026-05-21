package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"homedash/internal/earthquake"
	"homedash/internal/finance"
	"homedash/internal/holidays"
	"homedash/internal/sports"
	"homedash/internal/weather"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
)

// MCP JSON-RPC structures
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   interface{} `json:"error,omitempty"`
}

type MCPTool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema interface{} `json:"inputSchema"`
}

var (
	// Para SSE necesitamos trackear la respuesta activa
	sseMutex    sync.Mutex
	sseResponse http.ResponseWriter
)

func handleMCPSSE(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Enviar la URL del endpoint de mensajes según el estándar MCP
	fmt.Fprintf(w, "event: endpoint\ndata: /api/mcp/message\n\n")
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}

	sseMutex.Lock()
	sseResponse = w
	sseMutex.Unlock()

	// Mantener la conexión abierta
	<-r.Context().Done()
	
	sseMutex.Lock()
	sseResponse = nil
	sseMutex.Unlock()
}

func handleMCPMessage(w http.ResponseWriter, r *http.Request) {
	var req JSONRPCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Procesar de forma asíncrona y responder vía SSE
	go func() {
		processMCPRequestSSE(req)
	}()

	w.WriteHeader(http.StatusAccepted)
}

func processMCPRequestSSE(req JSONRPCRequest) {
	// Redirigir la salida de sendResponse y sendError al canal SSE si existe
	// Esta es una versión simplificada: en producción usarías un map de sesiones
	processMCPRequest(req)
}

// Modificamos sendResponse para que sea polimórfica (stdio o SSE)
func sendResponse(id interface{}, result interface{}) {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
	jsonBytes, _ := json.Marshal(resp)

	sseMutex.Lock()
	defer sseMutex.Unlock()

	if sseResponse != nil {
		fmt.Fprintf(sseResponse, "event: message\ndata: %s\n\n", jsonBytes)
		if f, ok := sseResponse.(http.Flusher); ok {
			f.Flush()
		}
	} else {
		fmt.Printf("%s\n", jsonBytes)
	}
}

func sendError(id interface{}, code int, message string, data interface{}) {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: map[string]interface{}{
			"code":    code,
			"message": message,
			"data":    data,
		},
	}
	jsonBytes, _ := json.Marshal(resp)

	sseMutex.Lock()
	defer sseMutex.Unlock()

	if sseResponse != nil {
		fmt.Fprintf(sseResponse, "event: message\ndata: %s\n\n", jsonBytes)
		if f, ok := sseResponse.(http.Flusher); ok {
			f.Flush()
		}
	} else {
		fmt.Printf("%s\n", jsonBytes)
	}
}

func handleMCP() {
	// Desactivar logs en stdout para no romper el protocolo JSON-RPC
	log.SetOutput(os.Stderr)

	reader := bufio.NewReader(os.Stdin)
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err != io.EOF {
				log.Printf("MCP Error leyendo stdin: %v", err)
			}
			break
		}

		var req JSONRPCRequest
		if err := json.Unmarshal(line, &req); err != nil {
			sendError(nil, -32700, "Parse error", nil)
			continue
		}

		processMCPRequest(req)
	}
}

func processMCPRequest(req JSONRPCRequest) {
	switch req.Method {
	case "initialize":
		sendResponse(req.ID, map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]interface{}{},
			"serverInfo": map[string]string{
				"name":    "homedash-mcp",
				"version": "1.0.0",
			},
		})
	case "notifications/initialized":
		// No response needed
	case "tools/list":
		sendResponse(req.ID, map[string]interface{}{
			"tools": []MCPTool{
				{
					Name:        "obtener_clima",
					Description: "Obtiene el clima actual y pronóstico para una ubicación (lat, lon).",
					InputSchema: map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"lat": map[string]string{"type": "string", "description": "Latitud (ej: -32.89)"},
							"lon": map[string]string{"type": "string", "description": "Longitud (ej: -68.82)"},
						},
					},
				},
				{
					Name:        "obtener_deportes",
					Description: "Obtiene información de deportes (Fútbol, F1, UFC).",
					InputSchema: map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
				},
				{
					Name:        "obtener_finanzas",
					Description: "Obtiene cotizaciones de Dólar, Cripto y Riesgo País.",
					InputSchema: map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
				},
				{
					Name:        "obtener_sismos",
					Description: "Obtiene los últimos sismos detectados por USGS e INPRES.",
					InputSchema: map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
				},
				{
					Name:        "obtener_feriados",
					Description: "Obtiene los feriados de Argentina para el año actual.",
					InputSchema: map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
				},
				{
					Name:        "obtener_todo",
					Description: "Obtiene todo el estado del dashboard de una sola vez.",
					InputSchema: map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
				},
			},
		})
	case "tools/call":
		var params struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		json.Unmarshal(req.Params, &params)
		handleToolCall(req.ID, params.Name, params.Arguments)
	default:
		sendError(req.ID, -32601, "Method not found", nil)
	}
}

func handleToolCall(id interface{}, name string, args json.RawMessage) {
	var result interface{}
	var err error

	switch name {
	case "obtener_clima":
		var a struct {
			Lat string `json:"lat"`
			Lon string `json:"lon"`
		}
		json.Unmarshal(args, &a)
		if a.Lat == "" || a.Lon == "" {
			settings := getDefaultSettings()
			a.Lat, a.Lon = settings.Lat, settings.Lon
		}
		result, err = weather.GetWeather(context.Background(), a.Lat, a.Lon)
	case "obtener_deportes":
		result = sports.GetSportsData()
	case "obtener_finanzas":
		result = finance.GetCachedFinance()
	case "obtener_sismos":
		result = earthquake.GetLatestEarthquakes()
	case "obtener_feriados":
		result = holidays.GetArgentinaHolidays()
	case "obtener_todo":
		settings := getDefaultSettings()
		wData, _ := weather.GetWeather(context.Background(), settings.Lat, settings.Lon)
		result = map[string]interface{}{
			"weather":     wData,
			"sports":      sports.GetSportsData(),
			"finance":     finance.GetCachedFinance(),
			"earthquakes": earthquake.GetLatestEarthquakes(),
			"holidays":    holidays.GetArgentinaHolidays(),
		}
	default:
		sendError(id, -32602, "Invalid tool name", nil)
		return
	}

	if err != nil {
		sendResponse(id, map[string]interface{}{
			"content": []map[string]interface{}{
				{"type": "text", "text": fmt.Sprintf("Error: %v", err)},
			},
			"isError": true,
		})
		return
	}

	jsonBytes, _ := json.MarshalIndent(result, "", "  ")
	sendResponse(id, map[string]interface{}{
		"content": []map[string]interface{}{
			{"type": "text", "text": string(jsonBytes)},
		},
	})
}

