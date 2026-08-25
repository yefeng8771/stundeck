package httpapi

import "net/http"

func (s *Server) diagnoseNetwork(w http.ResponseWriter, r *http.Request) {
	report := s.engine.DiagnoseNetwork(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{"diagnostic": report})
}
