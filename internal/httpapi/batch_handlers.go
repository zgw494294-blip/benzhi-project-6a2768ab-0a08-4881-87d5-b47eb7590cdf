package httpapi

import (
	"net/http"
	"seed-vault-admission/internal/admission"
)

func (a *API) CreateBatch(w http.ResponseWriter, r *http.Request) {
	var in admission.CreateBatchInput
	if err := decodeJSON(w, r, &in); err != nil {
		writeError(w, err)
		return
	}
	batch, err := a.service.CreateBatchContext(r.Context(), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, batch)
}

func (a *API) GetBatch(w http.ResponseWriter, r *http.Request) {
	detail, err := a.service.GetBatch(r.PathValue("batchID"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (a *API) UpdateSource(w http.ResponseWriter, r *http.Request) {
	var in admission.UpdateSourceInput
	if err := decodeJSON(w, r, &in); err != nil {
		writeError(w, err)
		return
	}
	batch, err := a.service.UpdateSource(r.PathValue("batchID"), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, batch)
}

func (a *API) GetPreflight(w http.ResponseWriter, r *http.Request) {
	preflight, err := a.service.SubmissionPreflight(r.PathValue("batchID"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, preflight)
}

func (a *API) AddPacket(w http.ResponseWriter, r *http.Request) {
	var in admission.AddPacketInput
	if err := decodeJSON(w, r, &in); err != nil {
		writeError(w, err)
		return
	}
	packet, err := a.service.AddPacket(r.PathValue("batchID"), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, packet)
}

func (a *API) UpdatePacket(w http.ResponseWriter, r *http.Request) {
	var in admission.UpdatePacketInput
	if err := decodeJSON(w, r, &in); err != nil {
		writeError(w, err)
		return
	}
	packet, err := a.service.UpdatePacket(r.PathValue("batchID"), r.PathValue("packetID"), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, packet)
}

func (a *API) DeletePacket(w http.ResponseWriter, r *http.Request) {
	var in admission.SimpleCommand
	if err := decodeJSON(w, r, &in); err != nil {
		writeError(w, err)
		return
	}
	batch, err := a.service.DeletePacket(r.PathValue("batchID"), r.PathValue("packetID"), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, batch)
}

func (a *API) SubmitBatch(w http.ResponseWriter, r *http.Request) {
	var in admission.SimpleCommand
	if err := decodeJSON(w, r, &in); err != nil {
		writeError(w, err)
		return
	}
	batch, err := a.service.SubmitBatch(r.PathValue("batchID"), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, batch)
}

func (a *API) AddAssessment(w http.ResponseWriter, r *http.Request) {
	var in admission.AddAssessmentInput
	if err := decodeJSON(w, r, &in); err != nil {
		writeError(w, err)
		return
	}
	test, err := a.service.AddAssessment(r.PathValue("batchID"), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, test)
}

func (a *API) GetConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"thresholds": a.service.Thresholds()})
}
