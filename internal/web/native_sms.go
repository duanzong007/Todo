package web

import (
	"encoding/json"
	"net/http"
	"strings"

	"todo/internal/service"
)

type NativeSMSPageData struct {
	CurrentUser *UserView
	ReturnPath  string
	AppTimeZone string
	UserID      string
}

type nativeSMSImportMessage struct {
	ID   string `json:"id"`
	Body string `json:"body"`
}

type nativeSMSImportRequest struct {
	Messages []nativeSMSImportMessage `json:"messages"`
}

type nativeSMSImportResponse struct {
	CreatedCount      int      `json:"created_count"`
	AcceptedIDs       []string `json:"accepted_ids"`
	UnsupportedIDs    []string `json:"unsupported_ids"`
	UnsupportedBodies []string `json:"unsupported_bodies"`
	Error             string   `json:"error,omitempty"`
}

func (h *Handler) handleNativeSMSPage(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(r)
	if !ok {
		h.redirectToLogin(w, r, "", "请先登录")
		return
	}

	w.Header().Set("Cache-Control", "no-store")

	data := NativeSMSPageData{
		CurrentUser: buildUserView(user),
		ReturnPath:  sanitizeReturnPath(r.URL.Query().Get("return")),
		AppTimeZone: h.location.String(),
		UserID:      user.ID.String(),
	}

	if err := h.templates.ExecuteTemplate(w, "native_sms.html", data); err != nil {
		http.Error(w, "render native sms page: "+err.Error(), http.StatusInternalServerError)
	}
}

func (h *Handler) handleNativeSMSImport(w http.ResponseWriter, r *http.Request) {
	user, ok := h.currentUser(r)
	if !ok {
		writeNativeSMSJSON(w, http.StatusUnauthorized, nativeSMSImportResponse{Error: "请先登录"})
		return
	}

	w.Header().Set("Cache-Control", "no-store")

	var request nativeSMSImportRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeNativeSMSJSON(w, http.StatusBadRequest, nativeSMSImportResponse{Error: "请求格式不正确"})
		return
	}

	candidates := make([]service.SMSCandidate, 0, len(request.Messages))
	for _, message := range request.Messages {
		body := strings.TrimSpace(message.Body)
		if body == "" {
			continue
		}
		candidates = append(candidates, service.SMSCandidate{
			ClientID: strings.TrimSpace(message.ID),
			Body:     body,
		})
	}

	if len(candidates) == 0 {
		writeNativeSMSJSON(w, http.StatusBadRequest, nativeSMSImportResponse{Error: "没有可提交的短信"})
		return
	}

	result, err := h.taskService.CreateFromSMSCandidates(r.Context(), user.ID, candidates)
	if err != nil {
		writeNativeSMSJSON(w, http.StatusBadRequest, nativeSMSImportResponse{Error: humanizeError(err)})
		return
	}

	if result.CreatedCount > 0 {
		h.publishDashboardUpdate(user.ID.String(), requestClientID(r))
	}

	writeNativeSMSJSON(w, http.StatusOK, nativeSMSImportResponse{
		CreatedCount:      result.CreatedCount,
		AcceptedIDs:       result.AcceptedClientIDs,
		UnsupportedIDs:    result.UnsupportedClientIDs,
		UnsupportedBodies: result.UnsupportedBodies,
	})
}

func writeNativeSMSJSON(w http.ResponseWriter, status int, payload nativeSMSImportResponse) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func sanitizeReturnPath(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "/"
	}
	if !strings.HasPrefix(trimmed, "/") || strings.HasPrefix(trimmed, "//") {
		return "/"
	}
	return trimmed
}
