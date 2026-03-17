package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
)

func main() {
	port := "8087"
	if p := os.Getenv("PORT"); p != "" {
		port = p
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST only", http.StatusMethodNotAllowed)
			return
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}

		prompt, _ := body["message"].(string) //nolint:errcheck // test helper
		resp := map[string]string{"response": fmt.Sprintf("I received your message: %s", prompt)}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp) //nolint:errcheck // test helper
	})

	addr := "127.0.0.1:" + port
	log.Printf("Test target listening on http://%s", addr) //nolint:forbidigo // standalone test helper
	log.Fatal(http.ListenAndServe(addr, nil))              //nolint:forbidigo,gosec // standalone test helper, no need for timeouts
}
