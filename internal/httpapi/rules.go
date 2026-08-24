package httpapi

import "net/http"

func (a *API) listOccupations(w http.ResponseWriter, _ *http.Request) {
	if a.occupations == nil {
		writeJSON(w, http.StatusOK, map[string]any{"items": []any{}})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": a.occupations.List()})
}
