package stx

import (
	"fmt"
	"log"
	"regexp"
	"sync"
	"time"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/build"
	"cuelang.org/go/cue/load"
	"cuelang.org/go/cue/parser"
	"github.com/logrusorgru/aurora"
)

type instanceHandler func(*build.Instance, *cue.Instance, cue.Value)

// GetBuildInstances loads and parses cue files and returns a list of build instances
func GetBuildInstances(args []string, pkg string) []*build.Instance {
	start := time.Now()
	const syntaxVersion = -1000 + 13

	config := load.Config{
		Package: pkg,
		Context: build.NewContext(
			build.ParseFile(func(name string, src interface{}) (*ast.File, error) {
				return parser.ParseFile(name, src,
					parser.FromVersion(syntaxVersion),
					parser.ParseComments,
				)
			})),
	}
	if len(args) < 1 {
		args = append(args, "./...")
	}
	// load finds files based on args and passes those to build
	// buildInstances is a list of build.Instances, each has been parsed
	buildInstances := load.Instances(args, &config)
	log.Printf("GetBuildInstances took %s\n", time.Since(start))
	return buildInstances
}

// Process iterates over instances applying the handler function for each
func Process(buildInstances []*build.Instance, exclude string, handler instanceHandler) {

	var excludeRegexp *regexp.Regexp
	var excludeRegexpErr error

	if exclude != "" {
		excludeRegexp, excludeRegexpErr = regexp.Compile(exclude)
		if excludeRegexpErr != nil {
			au := aurora.NewAurora(true) // TODO move to logger
			fmt.Println(au.Red(excludeRegexpErr.Error()))
		}
	}
	wg := new(sync.WaitGroup)
	for _, buildInstance := range buildInstances {
		if exclude != "" {
			if excludeRegexp.MatchString(buildInstance.DisplayPath) {
				continue
			}
		}
		// A cue instance defines a single configuration based on a collection of underlying CUE files.
		// cue.Build is designed to produce a single cue.Instance from n build.Instances
		// doing so however, would loose the connection between a stack and the build instance that
		// contains relevant path/file information related to the stack
		// here we cue.Build one at a time so we can maintain a 1:1:1:1 between
		// build.Instance, cue.Instance, cue.Value, and Stack
		wg.Add(1)
		go func(buildInstance *build.Instance, handler instanceHandler) {
			cueInstance := cue.Build([]*build.Instance{buildInstance})[0]
			if cueInstance.Err != nil {
				// parse errors will be exposed here
				fmt.Println(cueInstance.Err, cueInstance.Err.Position())
			} else {
				cueValue := cueInstance.Value()
				handler(buildInstance, cueInstance, cueValue)
			}
			wg.Done()
		}(buildInstance, handler)
	}
	wg.Wait()
}
