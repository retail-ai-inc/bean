{{ .ProjectObject.Copyright }}
package services

import (
	"context"
	// "github.com/getsentry/sentry-go"
	// "github.com/retail-ai-inc/bean/helpers"
	{{if .RepoExists}}"{{.ProjectObject.PkgPath}}/repositories"{{end}}
)

type {{.ServiceNameUpper}}Service interface {
	{{.ServiceNameUpper}}ServiceExampleFunc(ctx context.Context) (string, error)
}

{{if .RepoExists}}type {{.ServiceNameLower}}Service struct {
	{{.ServiceNameLower}}Repository repositories.{{.ServiceNameUpper}}Repository
}{{else}}type {{.ServiceNameLower}}Service struct{}{{end}}

{{if .RepoExists}}func New{{.ServiceNameUpper}}Service({{.ServiceNameLower}}Repo repositories.{{.ServiceNameUpper}}Repository) *{{.ServiceNameLower}}Service {
	return &{{.ServiceNameLower}}Service{{"{"}}{{.ServiceNameLower}}Repo{{"}"}}
}{{else}}func New{{.ServiceNameUpper}}Service() *{{.ServiceNameLower}}Service {
return &{{.ServiceNameLower}}Service{{"{}}"}}{{end}}

func (service *{{.ServiceNameLower}}Service) {{.ServiceNameUpper}}ServiceExampleFunc(ctx context.Context) (string, error) {
	// span := sentry.StartSpan(ctx, "http.service")
	// span.Description = helpers.CurrFuncName()
	// defer span.Finish()
	return "{{.ServiceNameUpper}}Service", nil
}
