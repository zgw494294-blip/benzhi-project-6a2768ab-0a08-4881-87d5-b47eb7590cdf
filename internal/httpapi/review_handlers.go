package httpapi

import (
	"net/http"
	"strconv"

	"seed-vault-admission/internal/admission"
)

func (a *API) SubmitRemediation(w http.ResponseWriter, r *http.Request) {
	var in admission.RemediationInput
	if err := decodeJSON(w, r, &in); err != nil {
		writeError(w, err)
		return
	}
	issue, err := a.service.SubmitRemediation(r.PathValue("batchID"), r.PathValue("issueID"), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, issue)
}

func (a *API) SubmitBatchRemediation(w http.ResponseWriter, r *http.Request) {
	var in admission.BatchRemediationInput
	if err := decodeJSON(w, r, &in); err != nil {
		writeError(w, err)
		return
	}
	result, err := a.service.SubmitBatchRemediation(r.PathValue("batchID"), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *API) ReviewIssue(w http.ResponseWriter, r *http.Request) {
	var in admission.ReviewIssueInput
	if err := decodeJSON(w, r, &in); err != nil {
		writeError(w, err)
		return
	}
	issue, err := a.service.ReviewIssue(r.PathValue("batchID"), r.PathValue("issueID"), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, issue)
}

func (a *API) ReviewBatch(w http.ResponseWriter, r *http.Request) {
	var in admission.ReviewBatchInput
	if err := decodeJSON(w, r, &in); err != nil {
		writeError(w, err)
		return
	}
	batch, err := a.service.ReviewBatch(r.PathValue("batchID"), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, batch)
}

func (a *API) FreezeBatch(w http.ResponseWriter, r *http.Request) {
	var in admission.FreezeInput
	if err := decodeJSON(w, r, &in); err != nil {
		writeError(w, err)
		return
	}
	batch, err := a.service.FreezeBatch(r.PathValue("batchID"), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, batch)
}

func (a *API) PreviewFreeze(w http.ResponseWriter, r *http.Request) {
	preview, err := a.service.PreviewFreeze(r.PathValue("batchID"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, preview)
}

func (a *API) GetReviewQueue(w http.ResponseWriter, r *http.Request) {
	allowed := map[string]bool{"speciesName": true, "owner": true, "severity": true, "issueStatus": true, "sort": true, "pageSize": true, "cursor": true}
	for key, values := range r.URL.Query() {
		if !allowed[key] {
			writeError(w, &admissionError{status: http.StatusBadRequest, code: "invalid_input", message: "未知查询参数: " + key})
			return
		}
		if len(values) != 1 {
			writeError(w, &admissionError{status: http.StatusBadRequest, code: "invalid_input", message: "查询参数不能重复: " + key})
			return
		}
	}
	pageSize := 0
	var err error
	if text := r.URL.Query().Get("pageSize"); text != "" {
		pageSize, err = strconv.Atoi(text)
		if err != nil {
			writeError(w, &admissionError{status: http.StatusBadRequest, code: "invalid_input", message: "pageSize 必须是整数"})
			return
		}
	}
	queue, err := a.service.SearchReviewQueue(admission.ReviewQueueQuery{SpeciesName: r.URL.Query().Get("speciesName"), Owner: r.URL.Query().Get("owner"), Severity: r.URL.Query().Get("severity"), IssueStatus: r.URL.Query().Get("issueStatus"), Sort: r.URL.Query().Get("sort"), PageSize: pageSize, Cursor: r.URL.Query().Get("cursor")})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, queue)
}
