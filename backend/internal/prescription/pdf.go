package prescription

import (
	"bytes"
	"fmt"
	"time"

	"github.com/go-pdf/fpdf"
)

// pdfData is everything the Rx PDF template needs. ClinicName/ClinicAddress/
// FooterText come from the shared invoice_template_placeholders table (see
// Service.Create) — never hardcoded, per CLAUDE.md's hard constraints.
type pdfData struct {
	ClinicName     string
	ClinicAddress  string
	FooterText     string
	PrescriptionID string
	PatientName    string
	ConsultantName string
	Content        string
	IssuedAt       time.Time
}

func renderPDF(d pdfData) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.AddPage()

	clinicName := d.ClinicName
	if clinicName == "" {
		clinicName = "Prescription"
	}
	pdf.SetFont("Helvetica", "B", 16)
	pdf.CellFormat(0, 10, clinicName, "", 1, "L", false, 0, "")

	if d.ClinicAddress != "" {
		pdf.SetFont("Helvetica", "", 10)
		pdf.CellFormat(0, 6, d.ClinicAddress, "", 1, "L", false, 0, "")
	}
	pdf.Ln(6)

	pdf.SetFont("Helvetica", "B", 12)
	pdf.CellFormat(0, 8, fmt.Sprintf("Prescription %s", d.PrescriptionID), "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 10)
	pdf.CellFormat(0, 6, fmt.Sprintf("Issued: %s", d.IssuedAt.Format("2006-01-02")), "", 1, "L", false, 0, "")
	pdf.CellFormat(0, 6, fmt.Sprintf("Patient: %s", d.PatientName), "", 1, "L", false, 0, "")
	pdf.CellFormat(0, 6, fmt.Sprintf("Consultant: %s", d.ConsultantName), "", 1, "L", false, 0, "")
	pdf.Ln(8)

	pdf.SetFont("Helvetica", "B", 10)
	pdf.CellFormat(0, 6, "Rx", "B", 1, "L", false, 0, "")
	pdf.Ln(2)
	pdf.SetFont("Helvetica", "", 11)
	pdf.MultiCell(0, 6, d.Content, "", "L", false)

	if d.FooterText != "" {
		pdf.Ln(10)
		pdf.SetFont("Helvetica", "I", 8)
		pdf.MultiCell(0, 5, d.FooterText, "", "L", false)
	}

	if err := pdf.Error(); err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
