package main

import (
	"fmt"
	"strings"
	"text/template"

	"github.com/jakewright/home-automation/tools/libraries/imports"
)

const packageDirRouter = "handler"

type routerDataEndpoint struct {
	NameUpper  string
	InputType  string
	OutputType string
	HTTPMethod string
	Path       string
}

type routerData struct {
	PackageName string
	Imports     []*imports.Imp
	Endpoints   []*routerDataEndpoint
}

const routerTemplateText = `// Code generated by jrpc. DO NOT EDIT.

package {{ .PackageName }}

{{ if .Imports }}
	import (
		{{- range .Imports }}
			{{ .Alias }} "{{ .Path }}"
		{{- end}}
	)
{{ end }}

// taxiRouter is an interface implemented by taxi.Router
type taxiRouter interface {
	HandleFunc(method, path string, handler func(context.Context, taxi.Decoder) (interface{}, error))
}

type handler interface {
	{{- range .Endpoints }}
		{{ .NameUpper }}(ctx context.Context, body *{{ .InputType }}) (*{{ .OutputType }}, error)
	{{- end }}
}

// RegisterRoutes adds the service's routes to the router
func RegisterRoutes(r taxiRouter, h handler) {
	{{ range .Endpoints -}}
		r.HandleFunc("{{ .HTTPMethod }}", "{{ .Path }}", func(ctx context.Context, decode taxi.Decoder) (interface{}, error) {
			body := &{{ .InputType }}{}
			if err := decode(body); err != nil {
				return nil, err
			}

			if err := body.Validate(); err != nil {
				return nil, err
			}

			return h.{{ .NameUpper }}(ctx, body)
		})

	{{ end -}}
}

// newHandler returns a handler that serves requests for
// this service. This is not exported as it is only used by
// tests. A service's main() function should create its own
// router (typically via bootstrap) and then use RegisterRoutes().
func newHandler(h handler) http.Handler {
	r := taxi.NewRouter()
	RegisterRoutes(r, h)
	return r
}
`

type routerGenerator struct {
	baseGenerator
}

func (g *routerGenerator) Template() (*template.Template, error) {
	return template.New("router_template").Parse(routerTemplateText)
}

func (g *routerGenerator) PackageDir() string {
	packageDir := packageDirRouter
	if g.options.RouterPackageName != "" {
		packageDir = g.options.RouterPackageName
	}
	return packageDir
}

func (g *routerGenerator) Data(im *imports.Manager) (interface{}, error) {
	// Don't generate anything if there's no service definition
	if g.file.Service == nil {
		return nil, nil
	}

	im.Add("context")
	im.Add("net/http")
	im.Add("github.com/jakewright/home-automation/libraries/go/taxi")

	// Make sure the service name is a suitable go struct name
	if ok := reValidGoStruct.MatchString(g.file.Service.Name); !ok {
		return "", fmt.Errorf("service name should be alphanumeric camelcase")
	}

	if g.file.Service == nil {
		return nil, nil
	}

	if len(g.file.Service.RPCs) == 0 {
		return nil, nil
	}

	endpoints := make([]*routerDataEndpoint, len(g.file.Service.RPCs))
	for i, r := range g.file.Service.RPCs {
		nameUpper := strings.ToUpper(r.Name[0:1]) + r.Name[1:]

		method, err := getMethod(r)
		if err != nil {
			return nil, fmt.Errorf("failed to get RPC %q method: %w", r.Name, err)
		}

		rpcPath, err := getPath(r)
		if err != nil {
			return nil, fmt.Errorf("failed to get RPC %q path: %w", r.Name, err)
		}

		inType, err := resolveTypeName(r.InputType, g.file, im)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve RPC %q input type: %w", r.Name, err)
		}

		outType, err := resolveTypeName(r.OutputType, g.file, im)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve RPC %q output type: %w", r.Name, err)
		}

		endpoints[i] = &routerDataEndpoint{
			NameUpper:  nameUpper,
			InputType:  inType.TypeName,
			OutputType: outType.TypeName,
			HTTPMethod: method,
			Path:       rpcPath,
		}
	}

	return &routerData{
		PackageName: g.PackageDir(), // This doesn't support separate package name to dir
		Imports:     im.Get(),
		Endpoints:   endpoints,
	}, nil
}

func (g *routerGenerator) Filename() string {
	return "router.go"
}
