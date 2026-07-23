package ingest

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"path/filepath"
	"sort"
	"strings"

	graph "github.com/mengshi02/codetrip/internal/model"
	"golang.org/x/tools/go/packages"
)

// GoSemanticStats records type-system facts added by the Go semantic pass.
type GoSemanticStats struct {
	Packages         int
	Calls            int
	Implements       int
	MethodImplements int
	Dispatches       int
	InterfaceMethods int
	UnresolvedCalls  int
	PackageNodes     int
	PackageImports   int
}

type goSemanticRefiner struct{}

func (goSemanticRefiner) Name() string { return "go-types" }

func (goSemanticRefiner) Supports(knowledgeGraph *graph.KnowledgeGraph) bool {
	supported := false
	knowledgeGraph.ForEachNode(func(node *graph.GraphNode) {
		if node.Properties.Language == "go" || strings.HasSuffix(node.Properties.FilePath, ".go") {
			supported = true
		}
	})
	return supported
}

func (goSemanticRefiner) Refine(repoPath string, knowledgeGraph *graph.KnowledgeGraph) (SemanticStats, error) {
	stats, err := ProcessGoSemantics(repoPath, knowledgeGraph)
	if err != nil {
		return SemanticStats{}, err
	}
	return SemanticStats{
		Refiner: "go-types", Language: "go", Units: stats.Packages,
		Facts: map[string]int{
			"CALLS": stats.Calls, "IMPLEMENTS": stats.Implements,
			"METHOD_IMPLEMENTS": stats.MethodImplements, "DISPATCHES_TO": stats.Dispatches,
			"IMPORTS": stats.PackageImports, "package_nodes": stats.PackageNodes,
			"interface_methods": stats.InterfaceMethods, "unresolved_calls": stats.UnresolvedCalls,
		},
	}, nil
}

type goCallableRange struct {
	start token.Pos
	end   token.Pos
	id    string
}

type goSemanticIndex struct {
	repoPath       string
	graph          *graph.KnowledgeGraph
	objects        map[types.Object]string
	positions      map[types.Object]token.Position
	named          map[*types.Named]string
	interfaceOwner map[*types.Func]string
	callables      map[string][]goCallableRange
	nodesByKey     map[string][]*graph.GraphNode
}

// ProcessGoSemantics uses the Go compiler's type information for facts that
// cannot be inferred reliably from syntax alone. Package load errors are
// tolerated when packages still expose usable syntax and TypesInfo.
func ProcessGoSemantics(repoPath string, g *graph.KnowledgeGraph) (GoSemanticStats, error) {
	absoluteRepoPath, err := filepath.Abs(repoPath)
	if err != nil {
		return GoSemanticStats{}, fmt.Errorf("resolve repository path: %w", err)
	}
	configuration := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
			packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo |
			packages.NeedImports | packages.NeedDeps | packages.NeedModule,
		Dir: absoluteRepoPath,
	}
	loaded, err := packages.Load(configuration, "./...")
	if err != nil {
		return GoSemanticStats{}, err
	}
	index := &goSemanticIndex{
		repoPath: absoluteRepoPath, graph: g,
		objects: make(map[types.Object]string), positions: make(map[types.Object]token.Position), named: make(map[*types.Named]string),
		interfaceOwner: make(map[*types.Func]string), callables: make(map[string][]goCallableRange),
		nodesByKey: make(map[string][]*graph.GraphNode),
	}
	index.indexGraphNodes()
	stats := GoSemanticStats{}
	for _, pkg := range loaded {
		if pkg.TypesInfo == nil || pkg.Fset == nil || len(pkg.Syntax) == 0 || !index.packageBelongsToRepository(pkg) {
			continue
		}
		stats.Packages++
		index.indexPackageDefinitions(pkg)
	}
	stats.PackageNodes, stats.PackageImports = index.addPackageGraph(loaded)
	stats.InterfaceMethods = index.addInterfaceMethods()
	implements, methodImplements, dispatches := index.addImplementations(loaded)
	stats.Implements, stats.MethodImplements, stats.Dispatches = implements, methodImplements, dispatches

	// Replace syntax-heuristic Go CALLS with compiler-resolved facts. Facts for
	// other languages remain untouched.
	goSources := make(map[string]struct{})
	g.ForEachNode(func(node *graph.GraphNode) {
		if _, semanticallyLoaded := index.callables[node.Properties.FilePath]; !semanticallyLoaded {
			return
		}
		if node.Properties.Language == "go" || strings.HasSuffix(node.Properties.FilePath, ".go") {
			goSources[node.ID] = struct{}{}
		}
	})
	g.RemoveRelationships(func(relationship *graph.GraphRelationship) bool {
		if relationship.Type != graph.RelCALLS {
			return false
		}
		_, ok := goSources[relationship.SourceID]
		return ok
	})
	for _, pkg := range loaded {
		if pkg.TypesInfo == nil || pkg.Fset == nil || len(pkg.Syntax) == 0 || !index.packageBelongsToRepository(pkg) {
			continue
		}
		resolved, unresolved := index.addCalls(pkg)
		stats.Calls += resolved
		stats.UnresolvedCalls += unresolved
	}
	return stats, nil
}

// addPackageGraph replaces Go's file-to-every-file import expansion with a
// compact persisted graph. The file-level resolution maps remain available to
// call resolution, while graph traversal uses Package --IMPORTS--> Package.
func (index *goSemanticIndex) addPackageGraph(loaded []*packages.Package) (int, int) {
	localPackages := make(map[string]*packages.Package)
	packageIDs := make(map[string]string)
	packageByFile := make(map[string]string)
	modulePath := ""

	for _, pkg := range loaded {
		if pkg.TypesInfo == nil || !index.packageBelongsToRepository(pkg) {
			continue
		}
		if pkg.Module != nil && pkg.Module.Path != "" {
			modulePath = pkg.Module.Path
		}
		localPackages[pkg.PkgPath] = pkg
		packageID := "Package:" + pkg.PkgPath
		packageIDs[pkg.PkgPath] = packageID
		directory := ""
		files := make([]string, 0, len(pkg.CompiledGoFiles))
		for _, filename := range pkg.CompiledGoFiles {
			path, ok := index.relativePath(filename)
			if !ok {
				continue
			}
			files = append(files, path)
			packageByFile["File:"+path] = packageID
			if directory == "" {
				directory = filepath.ToSlash(filepath.Dir(path))
				if directory == "." {
					directory = ""
				}
			}
		}
		index.graph.AddNode(&graph.GraphNode{
			ID: packageID, Label: graph.LabelPackage,
			Properties: graph.NodeProperties{
				Name: pkg.PkgPath, FilePath: directory, PackageName: pkg.Name, Language: "go",
			},
		})
		for _, path := range files {
			index.addRelationship(graph.RelCONTAINS, packageID, "File:"+path, 1, "go-package-membership")
		}
	}

	// packages.Load intentionally excludes files disabled by the current OS,
	// architecture, or build tags. Assign those Go files to a package by their
	// directory so a cross-platform source index does not retain File fan-out.
	goFiles := make([]*graph.GraphNode, 0)
	index.graph.ForEachNode(func(node *graph.GraphNode) {
		if node.Label == graph.LabelFile && strings.HasSuffix(node.Properties.FilePath, ".go") {
			goFiles = append(goFiles, node)
		}
	})
	for _, file := range goFiles {
		if packageByFile[file.ID] != "" {
			continue
		}
		directory := filepath.ToSlash(filepath.Dir(file.Properties.FilePath))
		if directory == "." {
			directory = ""
		}
		packagePath := modulePath
		if directory != "" {
			packagePath = strings.TrimSuffix(modulePath, "/") + "/" + directory
		}
		if packagePath == "" {
			packagePath = directory
		}
		packageID := "Package:" + packagePath
		packageByFile[file.ID] = packageID
		if packageIDs[packagePath] == "" {
			packageIDs[packagePath] = packageID
			packageName := filepath.Base(directory)
			if directory == "" {
				packageName = filepath.Base(packagePath)
			}
			index.graph.AddNode(&graph.GraphNode{
				ID: packageID, Label: graph.LabelPackage,
				Properties: graph.NodeProperties{
					Name: packagePath, FilePath: directory, PackageName: packageName, Language: "go",
				},
			})
		}
		index.addRelationship(graph.RelCONTAINS, packageID, file.ID, 1, "go-package-membership-fallback")
	}

	// Preserve imports found in build-tagged files by compacting their existing
	// file edges to package edges before removing the file representation.
	type packageImport struct{ source, target, reason string }
	fallbackImports := make([]packageImport, 0)
	index.graph.ForEachRelationship(func(relationship *graph.GraphRelationship) {
		if relationship.Type != graph.RelIMPORTS {
			return
		}
		source, target := packageByFile[relationship.SourceID], packageByFile[relationship.TargetID]
		if source != "" && target != "" && source != target {
			fallbackImports = append(fallbackImports, packageImport{source: source, target: target, reason: "go-package-import-fallback"})
		}
	})
	index.graph.RemoveRelationships(func(relationship *graph.GraphRelationship) bool {
		if relationship.Type != graph.RelIMPORTS {
			return false
		}
		return packageByFile[relationship.SourceID] != ""
	})

	imports := 0
	seenImports := make(map[string]struct{})
	for _, relationship := range fallbackImports {
		key := relationship.source + "\x00" + relationship.target
		if _, seen := seenImports[key]; seen {
			continue
		}
		seenImports[key] = struct{}{}
		index.addRelationship(graph.RelIMPORTS, relationship.source, relationship.target, 0.9, relationship.reason)
		imports++
	}
	for path, pkg := range localPackages {
		sourceID := packageIDs[path]
		for importedPath := range pkg.Imports {
			targetID := packageIDs[importedPath]
			if targetID == "" || targetID == sourceID {
				continue
			}
			key := sourceID + "\x00" + targetID
			index.addRelationship(graph.RelIMPORTS, sourceID, targetID, 1, "go-package-import")
			if _, seen := seenImports[key]; !seen {
				seenImports[key] = struct{}{}
				imports++
			}
		}
	}
	return len(packageIDs), imports
}

func (index *goSemanticIndex) packageBelongsToRepository(pkg *packages.Package) bool {
	for _, filename := range pkg.CompiledGoFiles {
		if _, ok := index.relativePath(filename); ok {
			return true
		}
	}
	return false
}

func (index *goSemanticIndex) relativePath(filename string) (string, bool) {
	relative, err := filepath.Rel(index.repoPath, filename)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", false
	}
	return filepath.ToSlash(relative), true
}

func semanticNodeKey(path, name string, label graph.NodeLabel) string {
	return path + "\x00" + name + "\x00" + string(label)
}

func (index *goSemanticIndex) indexGraphNodes() {
	index.graph.ForEachNode(func(node *graph.GraphNode) {
		index.nodesByKey[semanticNodeKey(node.Properties.FilePath, node.Properties.Name, node.Label)] =
			append(index.nodesByKey[semanticNodeKey(node.Properties.FilePath, node.Properties.Name, node.Label)], node)
	})
}

func (index *goSemanticIndex) findNode(path, name string, label graph.NodeLabel, line int) *graph.GraphNode {
	candidates := index.nodesByKey[semanticNodeKey(path, name, label)]
	if len(candidates) == 0 {
		return nil
	}
	for _, candidate := range candidates {
		if candidate.Properties.StartLine != nil && *candidate.Properties.StartLine == line {
			return candidate
		}
	}
	return candidates[0]
}

func (index *goSemanticIndex) indexPackageDefinitions(pkg *packages.Package) {
	for identifier, object := range pkg.TypesInfo.Defs {
		if object == nil || identifier == nil {
			continue
		}
		position := pkg.Fset.Position(object.Pos())
		index.positions[object] = position
		path, ok := index.relativePath(position.Filename)
		if !ok {
			continue
		}
		switch value := object.(type) {
		case *types.TypeName:
			named, ok := value.Type().(*types.Named)
			if !ok {
				continue
			}
			label := graph.LabelStruct
			switch named.Underlying().(type) {
			case *types.Interface:
				label = graph.LabelInterface
			case *types.Struct:
			default:
				continue
			}
			if node := index.findNode(path, value.Name(), label, position.Line-1); node != nil {
				index.objects[value] = node.ID
				index.named[named] = node.ID
				if iface, ok := named.Underlying().(*types.Interface); ok {
					iface.Complete()
					for methodIndex := 0; methodIndex < iface.NumMethods(); methodIndex++ {
						method := iface.Method(methodIndex)
						index.positions[method] = pkg.Fset.Position(method.Pos())
					}
				}
			}
		case *types.Func:
			label := graph.LabelFunction
			if signature, ok := value.Type().(*types.Signature); ok && signature.Recv() != nil {
				label = graph.LabelMethod
			}
			if node := index.findNode(path, value.Name(), label, position.Line-1); node != nil {
				index.objects[value] = node.ID
			}
		}
	}
	for _, file := range pkg.Syntax {
		filename := pkg.Fset.Position(file.Pos()).Filename
		path, ok := index.relativePath(filename)
		if !ok {
			continue
		}
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Name == nil {
				continue
			}
			object, _ := pkg.TypesInfo.Defs[function.Name].(*types.Func)
			id := index.objects[object]
			if id != "" {
				index.callables[path] = append(index.callables[path], goCallableRange{start: function.Pos(), end: function.End(), id: id})
			}
		}
	}
}

func (index *goSemanticIndex) addInterfaceMethods() int {
	added := 0
	for named, interfaceID := range index.named {
		iface, ok := named.Underlying().(*types.Interface)
		if !ok {
			continue
		}
		iface.Complete()
		if iface.NumMethods() == 0 {
			continue
		}
		for methodIndex := 0; methodIndex < iface.NumMethods(); methodIndex++ {
			method := iface.Method(methodIndex)
			if _, exists := index.objects[method]; exists {
				index.interfaceOwner[method] = interfaceID
				continue
			}
			position := index.positions[method]
			path, ok := index.relativePath(position.Filename)
			if !ok {
				continue
			}
			owner, ok := index.graph.GetNode(interfaceID)
			if !ok {
				continue
			}
			line := position.Line - 1
			end := line
			exported := method.Exported()
			node := &graph.GraphNode{
				ID: "Method:" + path + ":" + owner.Properties.Name + "." + method.Name(), Label: graph.LabelMethod,
				Properties: graph.NodeProperties{Name: method.Name(), FilePath: path, StartLine: &line, EndLine: &end, IsExported: &exported, Language: "go"},
			}
			index.graph.AddNode(node)
			index.nodesByKey[semanticNodeKey(path, method.Name(), graph.LabelMethod)] = append(index.nodesByKey[semanticNodeKey(path, method.Name(), graph.LabelMethod)], node)
			index.objects[method] = node.ID
			index.interfaceOwner[method] = interfaceID
			index.addRelationship(graph.RelHAS_METHOD, interfaceID, node.ID, 1, "go-types-interface-method")
			added++
		}
	}
	return added
}

func (index *goSemanticIndex) addImplementations(_ []*packages.Package) (int, int, int) {
	structs := make([]*types.Named, 0)
	interfaces := make([]*types.Named, 0)
	for named := range index.named {
		switch named.Underlying().(type) {
		case *types.Struct:
			structs = append(structs, named)
		case *types.Interface:
			interfaces = append(interfaces, named)
		}
	}
	sort.Slice(structs, func(i, j int) bool { return index.named[structs[i]] < index.named[structs[j]] })
	sort.Slice(interfaces, func(i, j int) bool { return index.named[interfaces[i]] < index.named[interfaces[j]] })
	implements, methodImplements, dispatches := 0, 0, 0
	for _, concrete := range structs {
		for _, abstract := range interfaces {
			iface := abstract.Underlying().(*types.Interface)
			iface.Complete()
			if iface.NumMethods() == 0 {
				continue
			}
			candidate := types.Type(concrete)
			pointer := false
			if !types.Implements(candidate, iface) {
				candidate = types.NewPointer(concrete)
				pointer = true
				if !types.Implements(candidate, iface) {
					continue
				}
			}
			concreteID, interfaceID := index.named[concrete], index.named[abstract]
			reason := "go-types-method-set"
			if pointer {
				reason = "go-types-pointer-method-set"
			}
			index.addRelationship(graph.RelIMPLEMENTS, concreteID, interfaceID, 1, reason)
			implements++
			for methodIndex := 0; methodIndex < iface.NumMethods(); methodIndex++ {
				interfaceMethod := iface.Method(methodIndex)
				concreteObject, _, _ := types.LookupFieldOrMethod(candidate, true, concrete.Obj().Pkg(), interfaceMethod.Name())
				concreteMethod, ok := concreteObject.(*types.Func)
				if !ok {
					continue
				}
				concreteMethodID, interfaceMethodID := index.objects[concreteMethod], index.objects[interfaceMethod]
				if concreteMethodID == "" || interfaceMethodID == "" || concreteMethodID == interfaceMethodID {
					continue
				}
				index.addRelationship(graph.RelMETHOD_IMPLEMENTS, concreteMethodID, interfaceMethodID, 1, reason)
				index.addRelationship(graph.RelDISPATCHES_TO, interfaceMethodID, concreteMethodID, 1, "go-types-interface-dispatch")
				methodImplements++
				dispatches++
			}
		}
	}
	return implements, methodImplements, dispatches
}

func (index *goSemanticIndex) addCalls(pkg *packages.Package) (int, int) {
	resolved, unresolved := 0, 0
	for _, file := range pkg.Syntax {
		position := pkg.Fset.Position(file.Pos())
		path, ok := index.relativePath(position.Filename)
		if !ok {
			continue
		}
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			caller := index.enclosingCallable(path, call.Pos())
			if caller == "" {
				unresolved++
				return true
			}
			callee := index.callTarget(pkg.TypesInfo, call.Fun)
			if callee == nil {
				return true
			}
			target := index.objects[callee]
			if target == "" {
				unresolved++
				return true
			}
			index.addRelationship(graph.RelCALLS, caller, target, 1, "go-types-call-target")
			resolved++
			return true
		})
	}
	return resolved, unresolved
}

func (index *goSemanticIndex) enclosingCallable(path string, position token.Pos) string {
	best := ""
	bestSize := int(^uint(0) >> 1)
	for _, candidate := range index.callables[path] {
		if position >= candidate.start && position <= candidate.end {
			size := int(candidate.end - candidate.start)
			if size < bestSize {
				best, bestSize = candidate.id, size
			}
		}
	}
	return best
}

func (index *goSemanticIndex) callTarget(info *types.Info, expression ast.Expr) *types.Func {
	switch value := expression.(type) {
	case *ast.Ident:
		function, _ := info.Uses[value].(*types.Func)
		return function
	case *ast.SelectorExpr:
		if selection := info.Selections[value]; selection != nil {
			function, _ := selection.Obj().(*types.Func)
			return function
		}
		function, _ := info.Uses[value.Sel].(*types.Func)
		return function
	case *ast.IndexExpr:
		return index.callTarget(info, value.X)
	case *ast.IndexListExpr:
		return index.callTarget(info, value.X)
	default:
		return nil
	}
}

func (index *goSemanticIndex) addRelationship(kind graph.RelationshipType, source, target string, confidence float64, reason string) {
	if source == "" || target == "" {
		return
	}
	index.graph.AddRelationship(&graph.GraphRelationship{
		ID: fmt.Sprintf("%s:%s->%s", kind, source, target), SourceID: source, TargetID: target,
		Type: kind, Confidence: confidence, Reason: reason,
	})
}
