package services

import (
	"context"

	// "github.com/retail-ai-inc/bean/v3/trace"
	{{if .Repos}}"{{.ProjectObject.PkgPath}}/repositories"{{end}}
)

type {{.ServiceNameUpper}}Service interface {
	{{.ServiceNameUpper}}ServiceExampleFunc(ctx context.Context) (string, error)
}

{{if .Repos}}type {{.ServiceNameLower}}Service struct {
{{- range .Repos}}
	{{.Lower}}Repository repositories.{{.Upper}}Repository
{{- end}}
}

func New{{.ServiceNameUpper}}Service({{range $i, $r := .Repos}}{{if $i}}, {{end}}{{$r.Lower}}Repo repositories.{{$r.Upper}}Repository{{end}}) *{{.ServiceNameLower}}Service {
	return &{{.ServiceNameLower}}Service{
{{- range .Repos}}
		{{.Lower}}Repository: {{.Lower}}Repo,
{{- end}}
	}
}{{else}}type {{.ServiceNameLower}}Service struct{}

func New{{.ServiceNameUpper}}Service() *{{.ServiceNameLower}}Service {
	return &{{.ServiceNameLower}}Service{}
}{{end}}

func (service *{{.ServiceNameLower}}Service) {{.ServiceNameUpper}}ServiceExampleFunc(ctx context.Context) (string, error) {
	// IMPORTANT: If you wanna trace the performance of your handler function then uncomment following 2 lines
	// _, finish := trace.StartSpan(ctx, "http.service")
	// defer finish()
	return "{{.ServiceNameUpper}}Service", nil
}
