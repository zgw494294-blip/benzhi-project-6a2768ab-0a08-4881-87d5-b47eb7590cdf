package httpapi

import (
	"net/http"

	"seed-vault-admission/internal/admission"
)

type API struct {
	service *admission.Service
	mux     *http.ServeMux
}

func New(service *admission.Service) *API {
	a := &API{service: service, mux: http.NewServeMux()}
	a.routes()
	return a
}

func (a *API) Handler() http.Handler {
	return securityHeaders(a.mux)
}

func (a *API) routes() {
	a.mux.HandleFunc("GET /", a.Home)
	a.mux.HandleFunc("GET /assets/{name}", a.Asset)
	a.mux.HandleFunc("GET /api/v1/config", a.GetConfig)
	a.mux.HandleFunc("POST /api/v1/batches", a.CreateBatch)
	a.mux.HandleFunc("GET /api/v1/batches/{batchID}", a.GetBatch)
	a.mux.HandleFunc("PATCH /api/v1/batches/{batchID}/source", a.UpdateSource)
	a.mux.HandleFunc("GET /api/v1/batches/{batchID}/preflight", a.GetPreflight)
	a.mux.HandleFunc("POST /api/v1/batches/{batchID}/packets", a.AddPacket)
	a.mux.HandleFunc("PUT /api/v1/batches/{batchID}/packets/{packetID}", a.UpdatePacket)
	a.mux.HandleFunc("PATCH /api/v1/batches/{batchID}/packets/{packetID}", a.UpdatePacket)
	a.mux.HandleFunc("DELETE /api/v1/batches/{batchID}/packets/{packetID}", a.DeletePacket)
	a.mux.HandleFunc("POST /api/v1/batches/{batchID}/submit", a.SubmitBatch)
	a.mux.HandleFunc("POST /api/v1/batches/{batchID}/assessments", a.AddAssessment)
	a.mux.HandleFunc("POST /api/v1/batches/{batchID}/issues/{issueID}/remediation", a.SubmitRemediation)
	a.mux.HandleFunc("POST /api/v1/batches/{batchID}/issues/remediation", a.SubmitBatchRemediation)
	a.mux.HandleFunc("POST /api/v1/batches/{batchID}/issues/{issueID}/review", a.ReviewIssue)
	a.mux.HandleFunc("POST /api/v1/batches/{batchID}/review", a.ReviewBatch)
	a.mux.HandleFunc("POST /api/v1/batches/{batchID}/freeze", a.FreezeBatch)
	a.mux.HandleFunc("GET /api/v1/batches/{batchID}/freeze-preview", a.PreviewFreeze)
	a.mux.HandleFunc("POST /api/v1/batches/{batchID}/certificate", a.IssueCertificate)
	a.mux.HandleFunc("GET /api/v1/review-queue", a.GetReviewQueue)
	a.mux.HandleFunc("GET /api/v1/certificates/{number}", a.VerifyCertificate)
	a.mux.HandleFunc("GET /certificates/{number}/print", a.CertificatePrint)
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self'; script-src 'self'; img-src 'self' data:")
		next.ServeHTTP(w, r)
	})
}
