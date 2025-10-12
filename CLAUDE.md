# gogen Project AI Assistant Guide

## Project Overview

**gogen** is a general-purpose Go code generation toolkit with zero third-party dependencies. It provides a fluent API for programmatically generating Go source code with full type checking capabilities, similar to the Go compiler itself.

### Key Features
- Zero third-party dependencies
- Full type checking during code generation
- Fluent API design for building Go code programmatically
- Support for Go generics and type parameters
- Template-based code generation
- Compatible with Go 1.19+

## Core Concepts

### 1. Package
The `Package` type represents a Go package being generated. It manages:
- Package metadata and imports
- Type definitions and declarations
- Global variables and constants
- Functions and methods

### 2. CodeBuilder
The `CodeBuilder` provides a fluent API for building code:
- Expression building (Val, VarVal, etc.)
- Statement generation (If, For, Switch, etc.)
- Function bodies and closures
- Type conversions and assertions

### 3. Type System
gogen maintains full type information:
- Built-in types via `types.Typ`
- Custom types and structs
- Generic type parameters
- Template types for code generation

## Development Workflow

### Building
```bash
go build -v ./...
```

### Testing
```bash
# Run all tests
go test -v ./...

# Run tests with race detection and coverage
go test -race -v -coverprofile="coverage.txt" -covermode=atomic ./...

# Test with type alias support (Go 1.23+)
GODEBUG=gotypesalias=1 go test -v ./...
```

### Running Examples
```bash
cd tutorial/01-Basic
go run main.go
```

## Code Patterns

### Creating a Package
```go
pkg := gogen.NewPackage("", "main", nil)
```

### Importing Packages
```go
fmt := pkg.Import("fmt")
```

### Creating Functions
```go
pkg.NewFunc(nil, "main", nil, nil, false).BodyStart(pkg).
    // Function body here
    End()
```

### Building Expressions
```go
cb.Val("Hello")          // String literal
cb.VarVal("x")           // Variable reference
cb.Val(pkg.Ref("func")) // Package reference
cb.Call(3)               // Function call with 3 args
```

### Variable Declarations
```go
// Short variable declaration: a, b := "Hi", 3
cb.DefineVarStart(token.NoPos, "a", "b").
    Val("Hi").Val(3).EndInit(2)

// var declaration: var c = b
cb.NewVarStart(nil, "c").VarVal("b").EndInit(1)
```

## Important Files

### Core Implementation
- `package.go:28-100`: Package configuration and initialization
- `codebuild.go:90-200`: CodeBuilder core methods
- `builtin.go:31-50`: Built-in type initialization
- `func.go:28-50`: Function parameter handling
- `template.go:30-50`: Template type system

### Key Types
- `Package`: Main package type managing code generation
- `CodeBuilder`: Fluent API for building code blocks
- `Param`: Function parameters (alias for types.Var)
- `Contract`: Interface for template type constraints
- `Recorder`: Interface for tracking object references

## Type Checking

gogen performs type checking during code generation:
- Invalid operations (e.g., `"Hello" + 1`) will be caught immediately
- Type inference for variable declarations
- Overload resolution for operators
- Generic type instantiation

## Debug Flags

Control debug output with `SetDebug()`:
```go
const (
    DbgFlagInstruction  // Instruction-level debugging
    DbgFlagImport       // Import tracking
    DbgFlagMatch        // Type matching
    DbgFlagComments     // Comment generation
    DbgFlagWriteFile    // File writing operations
    DbgFlagSetDebug     // Debug flag changes
    DbgFlagPersistCache // Cache persistence
    DbgFlagAll          // All flags
)
```

## CI/CD

The project uses GitHub Actions:
- Tests run on Ubuntu, Windows, and macOS
- Tests run against Go 1.19, 1.21, 1.22, and 1.23
- Separate test suite for type alias support (Go 1.23+)
- Code coverage tracked via Codecov

## Common Tasks

### Adding a New Feature
1. Identify the relevant core file (package.go, codebuild.go, etc.)
2. Follow existing patterns for fluent API methods
3. Add comprehensive tests in corresponding `*_test.go` files
4. Ensure type checking works correctly
5. Run full test suite before submitting

### Fixing Type Checking Issues
1. Check `builtin.go` for operator and type definitions
2. Review `codebuild.go` for expression building logic
3. Look at `typeutil/` for type utility functions
4. Add test cases in `builtin_test.go` or relevant test file

### Working with Templates/Generics
1. Understand `template.go` for template type system
2. Review `typeparams.go` for Go generics support
3. Check `internal/typeparams/` for generic-related utilities
4. Test with `typeparams_test.go`

## Testing Guidelines

- Test files follow `*_test.go` naming convention
- Major test files:
  - `builtin_test.go`: Built-in operations and types
  - `package_test.go`: Package-level features
  - `typeparams_test.go`: Generic types
  - `error_msg_test.go`: Error message formatting
  - `xgo_test.go`: Extended XGo features
- Always add tests for new features
- Ensure tests pass on all supported Go versions

## Best Practices

1. **Follow the fluent API pattern**: Chain methods for readability
2. **Maintain type safety**: Always use proper type information
3. **Handle errors gracefully**: Use the HandleErr callback in Config
4. **Document complex logic**: Especially for type inference
5. **Keep zero dependencies**: Don't add external dependencies
6. **Support multiple Go versions**: Test against 1.19, 1.21, 1.22, 1.23+

## Related Projects

This project is part of the XGo (goplus) ecosystem:
- Used by the XGo compiler for code generation
- Integrates with the XGo type system
- Supports XGo language extensions

## Useful Commands

```bash
# Install dependencies
go mod tidy

# Build the project
go build -v ./...

# Run specific test
go test -v -run TestName ./...

# Run tests with verbose output
go test -v ./...

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Format code
gofmt -s -w .

# Check for issues (if available)
go vet ./...
```

## Getting Help

- Check the tutorial examples in `tutorial/`
- Review test files for usage patterns
- Refer to Go's `go/types` package documentation
- Look at the XGo project documentation at https://goplus.org
