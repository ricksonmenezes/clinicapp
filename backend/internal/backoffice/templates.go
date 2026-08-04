// Package backoffice renders the staff-facing admin/clinician/attendant web
// pages (under /admin/*) that wrap around the JSON/HTMX-fragment API
// endpoints internal/patient, internal/consultant, internal/attendant,
// internal/service, internal/promo, internal/session, internal/invoice,
// internal/prescription, and internal/report already expose. Same
// philosophy as internal/portal: no business logic, no repository — pages
// are shells that call the existing API via HTMX (GET fragments) or a
// small client-side JSON-fetch helper (POST/PATCH forms), never a second
// copy of validation/authorization logic.
package backoffice

import (
	"html/template"
	"net/http"
	"path/filepath"
)

// pageNames is every content template paired with layout.html. Each is
// parsed as its own independent *template.Template (layout.html +
// {name}.html), same reasoning as internal/portal: html/template doesn't
// allow two files in one parse set to both {{define "content"}} without the
// last-parsed one silently winning for every page.
var pageNames = []string{
	"dashboard", "patients", "consultants", "attendants", "services",
	"packages", "sessions", "invoices", "prescriptions", "reports",
	"staff-new", "forbidden",
}

type Templates struct {
	pages map[string]*template.Template
}

// LoadTemplates parses every page in dir (which must contain layout.html
// plus one {name}.html per pageNames) once at startup.
func LoadTemplates(dir string) (*Templates, error) {
	layout := filepath.Join(dir, "layout.html")
	pages := make(map[string]*template.Template, len(pageNames))
	for _, name := range pageNames {
		tmpl, err := template.ParseFiles(layout, filepath.Join(dir, name+".html"))
		if err != nil {
			return nil, err
		}
		pages[name] = tmpl
	}
	return &Templates{pages: pages}, nil
}

// PageData is what every layout.html + content template pair renders with.
// Every /admin/* page requires authentication (enforced before Render is
// ever called), so unlike portal.PageData there's no Authenticated flag —
// Role is always populated and drives which nav links/forms a page shows.
type PageData struct {
	Title string
	Role  string
}

func (t *Templates) Render(w http.ResponseWriter, status int, page string, data PageData) {
	tmpl, ok := t.pages[page]
	if !ok {
		http.Error(w, "page not found", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_ = tmpl.ExecuteTemplate(w, "base", data)
}
