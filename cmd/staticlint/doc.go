// Command staticlint runs the project multichecker.
//
// staticlint aggregates several groups of analyzers:
//
//  1. Standard analyzers from golang.org/x/tools/go/analysis/passes.
//     They catch common correctness issues in Go code, for example:
//     - appends: detects missing values after append.
//     - assign: finds useless assignments like x = x.
//     - bools: validates boolean expressions.
//     - copylock: finds lock values copied by value.
//     - errorsas: verifies proper errors.As usage.
//     - loopclosure: finds references to loop variables from closures.
//     - printf: checks format strings and arguments.
//     - shift: validates suspicious bit shifts.
//     - structtag: validates struct field tags.
//     - unreachable: reports unreachable code.
//     - plus other built-in vet-style passes included in this command.
//
//  2. All SA-class analyzers from honnef.co/go/tools/staticcheck.
//     SA checks focus on correctness bugs and potentially broken behavior.
//
//  3. Additional non-SA staticcheck analyzers.
//     - S* (honnef.co/go/tools/simple): simplification and clarity checks.
//     - ST* (honnef.co/go/tools/stylecheck): style and API-usage conventions.
//
//  4. Extra public analyzers from third-party packages.
//     - nilerr: detects "return nil, err" style logic mistakes.
//     - forcetypeassert: finds unsafe type assertions that can panic.
//
//  5. Custom project analyzer noosexitinmain.
//     This analyzer forbids os.Exit, log.Fatal and panic calls outside
//     main.main in package main. Process termination and unrecoverable
//     failures should be handled in main.main; helper functions should
//     return errors instead.
//
// Run:
//
//	go run ./cmd/staticlint ./...
//
// You can also build and run the binary:
//
//	go build -o bin/staticlint ./cmd/staticlint
//	./bin/staticlint ./...
//
// To lint a specific package only:
//
//	go run ./cmd/staticlint ./cmd/shortener
package main
