// Package portal renders the patient-facing web pages (registration, login,
// dashboard, booking) that wrap around the JSON/HTMX-fragment API endpoints
// every other package already exposes. It owns no business logic and no
// repository — pages are shells that call the existing API via HTMX
// (GET fragments) or a small client-side JSON-fetch helper (POST forms),
// never a second copy of validation/authorization logic.
package portal

import (
	"html/template"
	"net/http"
	"path/filepath"
)

// pageNames is every content template paired with layout.html. Each is
// parsed as its own independent *template.Template (layout.html +
// {name}.html) rather than one shared set — html/template doesn't allow two
// files in the same set to both {{define "content"}}; parsing together
// would let the last-parsed page's content silently win for every page.
var pageNames = []string{
	"home", "register", "login", "check-email", "forgot-password", "reset-password", "dashboard", "book",
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
// Token is only used by reset-password (the query-string token, echoed into
// a hidden field); every other page ignores it.
type PageData struct {
	Title         string
	Authenticated bool
	Token         string
}

func (t *Templates) Render(w http.ResponseWriter, status int, page string, data PageData) {
	tmpl, ok := t.pages[page]
	if !ok {
		http.Error(w, "page not found", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	// Template execution errors here would mean a template bug, not a
	// request-specific failure — nothing left to do but let it surface in
	// the (already partially written) response, same as html/template's own
	// documented behavior for mid-render errors.
	_ = tmpl.ExecuteTemplate(w, "base", data)
}
