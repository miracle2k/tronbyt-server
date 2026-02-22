package renderer

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/tronbyt/pixlet/encode"
	"github.com/tronbyt/pixlet/runtime"
	"github.com/tronbyt/pixlet/runtime/modules/render_runtime/canvas"
	"github.com/tronbyt/pixlet/server/loader"
	"go.starlark.net/starlark"
	"golang.org/x/text/language"
)

// Render executes the Starlark script and returns the WebP image bytes.
func Render(
	ctx context.Context,
	path string,
	config map[string]any,
	width, height int,
	maxDuration time.Duration,
	timeout time.Duration,
	silenceOutput bool,
	output2x bool,
	timezone *string,
	locale *string,
	filters []string,
) ([]byte, []string, error) {
	location := time.Local
	if timezone != nil && *timezone != "" {
		v, err := time.LoadLocation(*timezone)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid timezone: %v", err)
		}
		location = v
	}

	lang := language.English
	if locale != nil && *locale != "" {
		var err error
		lang, err = language.Parse(*locale)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid locale: %v", err)
		}
	}

	var renderFilters encode.RenderFilters
	for _, f := range filters {
		var cf encode.ColorFilter
		if err := cf.UnmarshalText([]byte(f)); err == nil {
			renderFilters.ColorFilter = cf
		}
	}

	return loader.RenderApplet(
		path,
		config,
		loader.WithMeta(canvas.Metadata{
			Width:  int(width),
			Height: int(height),
			Is2x:   bool(output2x),
		}),
		loader.WithMaxDuration(maxDuration),
		loader.WithTimeout(timeout),
		loader.WithImageFormat(loader.ImageWebP),
		loader.WithSilenceOutput(silenceOutput),
		loader.WithLocation(location),
		loader.WithLanguage(lang),
		loader.WithFilters(renderFilters),
	)
}

// RenderSource executes Starlark source from memory and returns WebP image bytes.
func RenderSource(
	ctx context.Context,
	id string,
	src []byte,
	config map[string]any,
	width, height int,
	maxDuration time.Duration,
	timeout time.Duration,
	silenceOutput bool,
	output2x bool,
	timezone *string,
	locale *string,
	filters []string,
) ([]byte, []string, error) {
	location := time.Local
	if timezone != nil && *timezone != "" {
		v, err := time.LoadLocation(*timezone)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid timezone: %v", err)
		}
		location = v
	}

	lang := language.English
	if locale != nil && *locale != "" {
		var err error
		lang, err = language.Parse(*locale)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid locale: %v", err)
		}
	}

	meta := canvas.Metadata{
		Width:  width,
		Height: height,
		Is2x:   output2x,
	}

	opts := []runtime.AppletOption{
		runtime.WithCanvasMeta(meta),
		runtime.WithLocation(location),
		runtime.WithLanguage(lang),
	}

	var output []string
	if silenceOutput {
		opts = append(opts, runtime.WithPrintFunc(func(_ *starlark.Thread, msg string) {
			output = append(output, msg)
		}))
	}

	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeoutCause(ctx, timeout, fmt.Errorf("timeout after %s", timeout))
		defer cancel()
	}

	applet, err := runtime.NewApplet(id, src, opts...)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load applet: %w", err)
	}
	defer func() {
		if err := applet.Close(); err != nil {
			slog.Error("failed to close applet", "error", err)
		}
	}()

	renderFilters := encode.RenderFilters{Magnify: 1}
	for _, f := range filters {
		var cf encode.ColorFilter
		if err := cf.UnmarshalText([]byte(f)); err == nil {
			renderFilters.ColorFilter = cf
		}
	}

	if meta.Is2x && (applet.Manifest == nil || !applet.Manifest.Supports2x) {
		meta.Is2x = false
		renderFilters.Magnify *= 2
	}

	roots, err := applet.RunWithConfig(ctx, config)
	if err != nil {
		return nil, output, fmt.Errorf("error running script: %w", err)
	}

	screens := encode.ScreensFromRoots(roots, meta.ScaledWidth(), meta.ScaledHeight())

	filter := encode.ImageFilter(nil)
	var chain []encode.ImageFilter
	if renderFilters.Magnify > 1 {
		chain = append(chain, encode.Magnify(renderFilters.Magnify))
	}
	if imageFilter, err := renderFilters.ColorFilter.ImageFilter(); err == nil && imageFilter != nil {
		chain = append(chain, imageFilter)
	}
	if len(chain) > 0 {
		filter = encode.Chain(chain...)
	}

	if screens.ShowFullAnimation {
		maxDuration = 0
	}

	buf, err := screens.EncodeWebP(maxDuration, filter)
	if err != nil {
		return nil, output, fmt.Errorf("error rendering: %w", err)
	}

	return buf, output, nil
}

// GetSchema returns the schema JSON for the given script.
func GetSchema(path string, width, height int, output2x bool) ([]byte, error) {
	applet, err := runtime.NewAppletFromPath(
		path,
		runtime.WithCanvasMeta(canvas.Metadata{
			Width:  width,
			Height: height,
			Is2x:   bool(output2x),
		}),
	)
	if err != nil {
		return nil, err
	}
	defer func() {
		err := applet.Close()
		if err != nil {
			slog.Error("failed to close applet", "error", err)
		}
	}()

	if applet.Schema == nil {
		return []byte("{}"), nil
	}

	return json.Marshal(applet.Schema)
}

// CallSchemaHandler executes a schema handler function in the Starlark script.
func CallSchemaHandler(
	ctx context.Context,
	path string,
	config map[string]any,
	width, height int,
	output2x bool,
	handlerName string,
	parameter string,
) (string, error) {
	applet, err := runtime.NewAppletFromPath(
		path,
		runtime.WithCanvasMeta(canvas.Metadata{
			Width:  width,
			Height: height,
			Is2x:   bool(output2x),
		}),
	)
	if err != nil {
		return "", fmt.Errorf("failed to load applet from path: %w", err)
	}
	defer func() {
		err := applet.Close()
		if err != nil {
			slog.Error("failed to close applet", "error", err)
		}
	}()

	return applet.CallSchemaHandler(ctx, handlerName, parameter, config)
}
