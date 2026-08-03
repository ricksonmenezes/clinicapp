package invoice

import (
	"bytes"
	"fmt"
	"time"

	"github.com/go-pdf/fpdf"
)

// pdfData is everything the PDF template needs. Assembled by Service.Create
// from the underlying session/package, the patient, and the admin-editable
// placeholders — never hardcoded, per CLAUDE.md's hard constraints.
type pdfData struct {
	ClinicName    string
	ClinicAddress string
	FooterText    string
	InvoiceID     string
	PatientName   string
	LineItem      string
	Total         float64
	IssuedAt      time.Time
}

func renderPDF(d pdfData) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.AddPage()

	clinicName := d.ClinicName
	if clinicName == "" {
		clinicName = "Invoice"
	}
	pdf.SetFont("Helvetica", "B", 16)
	pdf.CellFormat(0, 10, clinicName, "", 1, "L", false, 0, "")

	if d.ClinicAddress != "" {
		pdf.SetFont("Helvetica", "", 10)
		pdf.CellFormat(0, 6, d.ClinicAddress, "", 1, "L", false, 0, "")
	}
	pdf.Ln(6)

	pdf.SetFont("Helvetica", "B", 12)
	pdf.CellFormat(0, 8, fmt.Sprintf("Invoice %s", d.InvoiceID), "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 10)
	pdf.CellFormat(0, 6, fmt.Sprintf("Issued: %s", d.IssuedAt.Format("2006-01-02")), "", 1, "L", false, 0, "")
	pdf.CellFormat(0, 6, fmt.Sprintf("Patient: %s", d.PatientName), "", 1, "L", false, 0, "")
	pdf.Ln(6)

	pdf.SetFont("Helvetica", "B", 10)
	pdf.CellFormat(140, 8, "Description", "B", 0, "L", false, 0, "")
	pdf.CellFormat(50, 8, "Amount", "B", 1, "R", false, 0, "")
	pdf.SetFont("Helvetica", "", 10)
	pdf.CellFormat(140, 8, d.LineItem, "", 0, "L", false, 0, "")
	pdf.CellFormat(50, 8, fmt.Sprintf("%.2f", d.Total), "", 1, "R", false, 0, "")
	pdf.Ln(4)

	pdf.SetFont("Helvetica", "B", 10)
	pdf.CellFormat(140, 8, "Total", "T", 0, "L", false, 0, "")
	pdf.CellFormat(50, 8, fmt.Sprintf("%.2f", d.Total), "T", 1, "R", false, 0, "")

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
