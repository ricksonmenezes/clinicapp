package integration

import (
	"bytes"
	"net/http"
	"testing"
)

func createSession(t *testing.T, baseURL, adminToken string, body map[string]any) map[string]any {
	t.Helper()
	var created map[string]any
	resp := PostJSONAuth(t, baseURL, "/sessions", "mobile", adminToken, body, &created)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create session: want 201, got %d (%v)", resp.StatusCode, created)
	}
	return created
}

func TestInvoiceCreate_FromSession(t *testing.T) {
	ts, fakeMailer, _ := NewTestServer(t)
	adminToken := LoginAdmin(t, ts.URL)
	patientID, consultantID, _, serviceID := sessionFixture(t, ts.URL, fakeMailer, adminToken, "invsession", 25, true)

	sess := createSession(t, ts.URL, adminToken, map[string]any{
		"patient_id": patientID, "service_id": serviceID, "consultant_id": consultantID,
		"scheduled_at": "2026-08-10T10:00:00Z",
	})
	sessionID, _ := sess["id"].(string)

	var created map[string]any
	resp := PostJSONAuth(t, ts.URL, "/invoices", "mobile", adminToken, map[string]any{
		"session_id": sessionID,
	}, &created)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create invoice: want 201, got %d (%v)", resp.StatusCode, created)
	}
	if created["patient_id"] != patientID {
		t.Fatalf("expected patient_id %s, got %v", patientID, created["patient_id"])
	}
	if created["total"] != float64(500) {
		t.Fatalf("expected total 500, got %v", created["total"])
	}
	if created["pdf_available"] != true {
		t.Fatalf("expected pdf_available true, got %v", created["pdf_available"])
	}
	invoiceID, _ := created["id"].(string)

	req, err := http.NewRequest(http.MethodGet, ts.URL+"/invoices/"+invoiceID+"/pdf", nil)
	if err != nil {
		t.Fatalf("build pdf request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("X-Client-Type", "mobile")
	pdfResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("download pdf: %v", err)
	}
	defer pdfResp.Body.Close()
	if pdfResp.StatusCode != http.StatusOK {
		t.Fatalf("download pdf: want 200, got %d", pdfResp.StatusCode)
	}
	if ct := pdfResp.Header.Get("Content-Type"); ct != "application/pdf" {
		t.Fatalf("expected Content-Type application/pdf, got %q", ct)
	}
	buf := new(bytes.Buffer)
	buf.ReadFrom(pdfResp.Body)
	if !bytes.HasPrefix(buf.Bytes(), []byte("%PDF-")) {
		t.Fatalf("expected PDF magic header, got %q", buf.Bytes()[:min(20, buf.Len())])
	}
}

func TestInvoiceCreate_FromPackage(t *testing.T) {
	ts, fakeMailer, _ := NewTestServer(t)
	adminToken := LoginAdmin(t, ts.URL)
	patientID, _, _, serviceID := sessionFixture(t, ts.URL, fakeMailer, adminToken, "invpackage", 25, true)

	var pkg map[string]any
	PostJSONAuth(t, ts.URL, "/packages", "mobile", adminToken, map[string]any{
		"service_id": serviceID, "name": "Invoice pack", "session_count": 4, "price": 1800,
	}, &pkg)
	packageID, _ := pkg["id"].(string)

	var subscribed map[string]any
	PostJSONAuth(t, ts.URL, "/patient-packages", "mobile", adminToken, map[string]any{
		"patient_id": patientID, "package_id": packageID,
	}, &subscribed)
	patientPackageID, _ := subscribed["id"].(string)

	var created map[string]any
	resp := PostJSONAuth(t, ts.URL, "/invoices", "mobile", adminToken, map[string]any{
		"package_id": patientPackageID,
	}, &created)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create invoice: want 201, got %d (%v)", resp.StatusCode, created)
	}
	if created["total"] != float64(1800) {
		t.Fatalf("expected total 1800, got %v", created["total"])
	}
	if created["patient_id"] != patientID {
		t.Fatalf("expected patient_id %s, got %v", patientID, created["patient_id"])
	}
}

func TestInvoiceCreate_RejectsNeitherOrBothSessionAndPackage(t *testing.T) {
	ts, fakeMailer, _ := NewTestServer(t)
	adminToken := LoginAdmin(t, ts.URL)
	patientID, consultantID, _, serviceID := sessionFixture(t, ts.URL, fakeMailer, adminToken, "invneither", 25, true)

	resp := PostJSONAuth(t, ts.URL, "/invoices", "mobile", adminToken, map[string]any{}, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("create invoice with neither: want 400, got %d", resp.StatusCode)
	}

	sess := createSession(t, ts.URL, adminToken, map[string]any{
		"patient_id": patientID, "service_id": serviceID, "consultant_id": consultantID,
		"scheduled_at": "2026-08-10T10:00:00Z",
	})
	sessionID, _ := sess["id"].(string)

	var pkg map[string]any
	PostJSONAuth(t, ts.URL, "/packages", "mobile", adminToken, map[string]any{
		"service_id": serviceID, "name": "Both pack", "session_count": 2, "price": 900,
	}, &pkg)
	packageID, _ := pkg["id"].(string)
	var subscribed map[string]any
	PostJSONAuth(t, ts.URL, "/patient-packages", "mobile", adminToken, map[string]any{
		"patient_id": patientID, "package_id": packageID,
	}, &subscribed)
	patientPackageID, _ := subscribed["id"].(string)

	resp = PostJSONAuth(t, ts.URL, "/invoices", "mobile", adminToken, map[string]any{
		"session_id": sessionID, "package_id": patientPackageID,
	}, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("create invoice with both: want 400, got %d", resp.StatusCode)
	}
}

func TestInvoiceCreate_RejectsUnknownSessionOrPackage(t *testing.T) {
	ts, _, _ := NewTestServer(t)
	adminToken := LoginAdmin(t, ts.URL)

	resp := PostJSONAuth(t, ts.URL, "/invoices", "mobile", adminToken, map[string]any{
		"session_id": "00000000-0000-0000-0000-000000000000",
	}, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("create invoice for unknown session: want 400, got %d", resp.StatusCode)
	}

	resp = PostJSONAuth(t, ts.URL, "/invoices", "mobile", adminToken, map[string]any{
		"package_id": "00000000-0000-0000-0000-000000000000",
	}, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("create invoice for unknown package: want 400, got %d", resp.StatusCode)
	}
}

func TestInvoiceTemplatePlaceholders_UpsertAndList(t *testing.T) {
	ts, _, _ := NewTestServer(t)
	adminToken := LoginAdmin(t, ts.URL)

	req, err := http.NewRequest(http.MethodPut, ts.URL+"/invoice-template-placeholders/clinic_name", bytes.NewReader([]byte(`{"value":"Acme Clinic"}`)))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Client-Type", "mobile")
	req.Header.Set("Authorization", "Bearer "+adminToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("set placeholder: want 200, got %d", resp.StatusCode)
	}

	var listResp map[string]any
	getResp := Get(t, ts.URL, "/invoice-template-placeholders", "mobile", adminToken, &listResp)
	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("list placeholders: want 200, got %d", getResp.StatusCode)
	}
	items, _ := listResp["placeholders"].([]any)
	if len(items) != 1 {
		t.Fatalf("expected 1 placeholder, got %d (%v)", len(items), items)
	}
	first, _ := items[0].(map[string]any)
	if first["key"] != "clinic_name" || first["value"] != "Acme Clinic" {
		t.Fatalf("expected clinic_name=Acme Clinic, got %v", first)
	}
}

func TestConsultantCommissionHistory(t *testing.T) {
	ts, fakeMailer, _ := NewTestServer(t)
	adminToken := LoginAdmin(t, ts.URL)
	patientID, consultantID, _, serviceID := sessionFixture(t, ts.URL, fakeMailer, adminToken, "commhistory", 25, true)

	createSession(t, ts.URL, adminToken, map[string]any{
		"patient_id": patientID, "service_id": serviceID, "consultant_id": consultantID,
		"scheduled_at": "2026-08-10T10:00:00Z",
	})
	createSession(t, ts.URL, adminToken, map[string]any{
		"patient_id": patientID, "service_id": serviceID, "consultant_id": consultantID,
		"scheduled_at": "2026-08-11T10:00:00Z", "commission_override": 55,
	})

	var history map[string]any
	resp := Get(t, ts.URL, "/consultants/"+consultantID+"/commission-history", "mobile", adminToken, &history)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("commission history: want 200, got %d", resp.StatusCode)
	}
	items, _ := history["commission_history"].([]any)
	if len(items) != 2 {
		t.Fatalf("expected 2 commission history entries, got %d (%v)", len(items), items)
	}

	sources := map[string]bool{}
	for _, item := range items {
		entry, _ := item.(map[string]any)
		sources[entry["resolution_source"].(string)] = true
	}
	if !sources["consultant_default"] || !sources["session_override"] {
		t.Fatalf("expected both consultant_default and session_override entries, got %v", sources)
	}

	resp = Get(t, ts.URL, "/consultants/00000000-0000-0000-0000-000000000000/commission-history", "mobile", adminToken, nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("commission history for unknown consultant: want 404, got %d", resp.StatusCode)
	}
}
