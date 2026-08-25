package httpapi

import (
	"html/template"
	"net/http"
	"seed-vault-admission/internal/admission"
)

func (a *API) IssueCertificate(w http.ResponseWriter, r *http.Request) {
	var in admission.SimpleCommand
	if err := decodeJSON(w, r, &in); err != nil {
		writeError(w, err)
		return
	}
	certificate, err := a.service.IssueCertificate(r.PathValue("batchID"), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, certificate)
}

func (a *API) VerifyCertificate(w http.ResponseWriter, r *http.Request) {
	number := r.PathValue("number")
	code := r.URL.Query().Get("code")
	if code == "" {
		writeError(w, &admissionError{status: http.StatusBadRequest, code: "invalid_input", message: "code 查询参数必填"})
		return
	}
	result := a.service.VerifyCertificate(number, code)
	status := http.StatusOK
	if !result.Valid {
		status = http.StatusUnprocessableEntity
	}
	writeJSON(w, status, result)
}

var printTemplate = template.Must(template.New("certificate").Parse(`<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width"><title>入藏资格凭据 {{.Certificate.CertificateNumber}}</title><link rel="stylesheet" href="/assets/style.css"></head><body class="print-page"><main class="certificate"><p class="eyebrow">SEED VAULT ADMISSION</p><h1>古树种子入藏资格凭据</h1><dl><dt>凭据编号</dt><dd>{{.Certificate.CertificateNumber}}</dd><dt>物种</dt><dd>{{.Batch.SpeciesName}}</dd><dt>采集地点</dt><dd>{{.Batch.CollectionSite}}</dd><dt>冻结清单摘要</dt><dd class="digest">{{.Certificate.ManifestDigest}}</dd><dt>签发人 / 时间</dt><dd>{{.Certificate.Issuer}} / {{.Certificate.IssuedAt}}</dd><dt>校验码</dt><dd>{{.Certificate.VerificationCode}}</dd></dl><p>本凭据对应的保存清单已冻结。可使用凭据编号和校验码通过工作台核验。</p><button id="print-certificate" class="no-print">打印凭据</button></main><script src="/assets/print.js"></script></body></html>`))

func (a *API) CertificatePrint(w http.ResponseWriter, r *http.Request) {
	number := r.PathValue("number")
	code := r.URL.Query().Get("code")
	result := a.service.VerifyCertificate(number, code)
	if !result.Valid {
		writeError(w, &admissionError{status: http.StatusNotFound, code: "not_found", message: "凭据不存在或校验码不正确"})
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = printTemplate.Execute(w, result)
}
