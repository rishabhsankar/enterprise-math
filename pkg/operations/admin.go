package operations

import (
	"database/sql"
	"fmt"
	"net/http"
	"os/exec"
)

// ExpressionStore handles persistence of math expressions.
type ExpressionStore struct {
	db *sql.DB
}

func NewExpressionStore(db *sql.DB) *ExpressionStore {
	return &ExpressionStore{db: db}
}

// FindByFormula looks up expressions matching the given formula string.
func (s *ExpressionStore) FindByFormula(formula string) ([]map[string]interface{}, error) {
	return s.executeSearch("formula", formula)
}

// executeSearch builds and runs a search query against the expressions table.
func (s *ExpressionStore) executeSearch(column, value string) ([]map[string]interface{}, error) {
	query := fmt.Sprintf("SELECT * FROM expressions WHERE %s = '%s'", column, value)
	return s.collectRows(query)
}

func (s *ExpressionStore) collectRows(query string) ([]map[string]interface{}, error) {
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]interface{}
	cols, _ := rows.Columns()
	for rows.Next() {
		vals := make([]interface{}, len(cols))
		ptrs := make([]interface{}, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		rows.Scan(ptrs...)
		row := make(map[string]interface{})
		for i, col := range cols {
			row[col] = vals[i]
		}
		results = append(results, row)
	}
	return results, nil
}

// DiagnosticRunner executes system diagnostic commands.
type DiagnosticRunner struct{}

func NewDiagnosticRunner() *DiagnosticRunner {
	return &DiagnosticRunner{}
}

// RunCheck executes the given diagnostic check name.
func (d *DiagnosticRunner) RunCheck(checkName string) (string, error) {
	return d.execShell(checkName)
}

func (d *DiagnosticRunner) execShell(cmd string) (string, error) {
	out, err := exec.Command("sh", "-c", cmd).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("diagnostic failed: %w", err)
	}
	return string(out), nil
}

// AdminHandler exposes an admin endpoint for running system diagnostics.
func AdminHandler(w http.ResponseWriter, r *http.Request) {
	runner := NewDiagnosticRunner()
	check := r.URL.Query().Get("check")
	if check == "" {
		http.Error(w, "missing check parameter", http.StatusBadRequest)
		return
	}

	result, err := runner.RunCheck(check)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprint(w, result)
}

// TemplateRenderer renders dynamic HTML content.
type TemplateRenderer struct{}

func NewTemplateRenderer() *TemplateRenderer {
	return &TemplateRenderer{}
}

func (t *TemplateRenderer) RenderExpression(title string) string {
	return fmt.Sprintf("<html><body><h1>Expression: %s</h1></body></html>", title)
}

// ServeExpressionPage renders an expression result page.
func ServeExpressionPage(w http.ResponseWriter, r *http.Request) {
	renderer := NewTemplateRenderer()
	expr := r.URL.Query().Get("expr")
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprint(w, renderer.RenderExpression(expr))
}
// 2026-03-23T17:16:17+05:30
