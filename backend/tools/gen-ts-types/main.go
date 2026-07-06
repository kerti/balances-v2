// Command gen-ts-types regenerates frontend/src/api/generated.types.ts, a
// structural mirror of the sqlc-generated + repo wire-facing Go structs
// (issue #365). Field names, presence, and null-vs-non-null shape are
// derived directly from the Go source, closing the drift the hand-maintained
// frontend/src/api/types.ts used to be exposed to — a changed nullability or
// an added/removed column used to sail through silently.
//
// It deliberately does NOT attempt to reproduce sqlc's enum-shaped columns
// (subtype, status, ownership_type, ...) as TypeScript string-literal
// unions: sqlc collapses every CHECK-constraint column to a plain Go
// `string`, so there is no Go-side enum type to read the literal values from.
// Those fields come out as plain `string`/`string | null` here; frontend/src/
// api/types.ts re-narrows the specific enum fields to literal unions via
// `Omit<Generated.X, "field"> & { field: "a" | "b" }` — see its header.
//
// It also does NOT walk the whole internal/db package: that package also
// holds several internal-only rows never meant to reach the wire (User has
// google_sub, PasswordResetToken has token_hash, plus Session, Household,
// HouseholdInvitation, LocalCredential, OnboardingHandshake, MonthlyReport —
// the last one's JSONB columns get reserialised by hand in
// internal/reports). Instead this tool works off the explicit `targets`
// allowlist below.
//
//	cd backend && go run ./tools/gen-ts-types          # regenerate
//	cd backend && go run ./tools/gen-ts-types -check   # CI: fail if stale
package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
)

// target names one Go struct to mirror into the generated TS file.
type target struct {
	file       string // path relative to backend/
	structName string // Go struct name
	tsName     string // exported TS type name (differs for the *Detail -> *Details rename)

	// nullableFields lists json field names (not Go names) whose TS type
	// should be forced nullable regardless of the Go field's own
	// pointer-ness — currently only Tag's deleted_at, a soft-delete column
	// stored as a non-pointer pgtype.Timestamptz that IS exposed on the
	// wire (unlike every other table's deleted_at, see skip below).
	nullableFields map[string]bool
}

// deleted_at is dropped by default: every wire-facing struct except Tag
// omits its soft-delete column entirely (a deleted row is never returned in
// the first place, so callers have no use for the column). Tag is the one
// exception — list nullableFields["deleted_at"]=true to keep it.
const defaultSkippedField = "deleted_at"

var targets = []target{
	{file: "internal/db/models.go", structName: "Asset", tsName: "Asset"},
	{file: "internal/db/models.go", structName: "AssetSnapshot", tsName: "AssetSnapshot"},
	{file: "internal/db/models.go", structName: "Tag", tsName: "Tag", nullableFields: map[string]bool{"deleted_at": true}},
	{file: "internal/db/models.go", structName: "FxRate", tsName: "FxRate"},
	{file: "internal/db/models.go", structName: "Income", tsName: "Income"},
	{file: "internal/db/models.go", structName: "Investment", tsName: "Investment"},
	{file: "internal/db/models.go", structName: "InvestmentSnapshot", tsName: "InvestmentSnapshot"},
	{file: "internal/db/models.go", structName: "InvestmentTransaction", tsName: "InvestmentTransaction"},
	{file: "internal/db/models.go", structName: "Liability", tsName: "Liability"},
	{file: "internal/db/models.go", structName: "LiabilitySnapshot", tsName: "LiabilitySnapshot"},
	{file: "internal/db/models.go", structName: "Receivable", tsName: "Receivable"},
	{file: "internal/db/models.go", structName: "ReceivableSnapshot", tsName: "ReceivableSnapshot"},
	{file: "internal/db/models.go", structName: "BankAccountDetail", tsName: "BankAccountDetails"},
	{file: "internal/db/models.go", structName: "PropertyDetail", tsName: "PropertyDetails"},
	{file: "internal/db/models.go", structName: "VehicleDetail", tsName: "VehicleDetails"},
	{file: "internal/db/models.go", structName: "StockDetail", tsName: "StockDetails"},
	{file: "internal/db/models.go", structName: "MutualFundDetail", tsName: "MutualFundDetails"},
	{file: "internal/db/models.go", structName: "GoldDetail", tsName: "GoldDetails"},
	{file: "internal/db/models.go", structName: "BondDetail", tsName: "BondDetails"},
	{file: "internal/db/models.go", structName: "TimeDepositDetail", tsName: "TimeDepositDetails"},
	{file: "internal/repo/time_deposits.go", structName: "RolloverRef", tsName: "RolloverRef"},
}

const outPath = "../frontend/src/api/generated.types.ts"

func main() {
	check := flag.Bool("check", false, "compare freshly generated output against the committed file; exit 1 if stale, without writing")
	flag.Parse()

	out, err := generate()
	if err != nil {
		fmt.Fprintln(os.Stderr, "gen-ts-types:", err)
		os.Exit(1)
	}

	if *check {
		existing, err := os.ReadFile(outPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "gen-ts-types: %s not found — run `make backend-gen-ts-types`\n", outPath)
			os.Exit(1)
		}
		if !bytes.Equal(existing, out) {
			fmt.Fprintf(os.Stderr, "gen-ts-types: %s is stale — run `make backend-gen-ts-types` and commit the result\n", outPath)
			os.Exit(1)
		}
		fmt.Println("gen-ts-types: up to date")
		return
	}

	if err := os.WriteFile(outPath, out, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "gen-ts-types:", err)
		os.Exit(1)
	}
	fmt.Println("gen-ts-types: wrote", outPath)
}

func generate() ([]byte, error) {
	fset := token.NewFileSet()
	parsed := map[string]*ast.File{}

	var buf bytes.Buffer
	buf.WriteString(header)

	for _, tgt := range targets {
		file, ok := parsed[tgt.file]
		if !ok {
			f, err := parser.ParseFile(fset, tgt.file, nil, parser.ParseComments)
			if err != nil {
				return nil, fmt.Errorf("parsing %s: %w", tgt.file, err)
			}
			parsed[tgt.file] = f
			file = f
		}

		fields, err := renderStruct(file, tgt)
		if err != nil {
			return nil, fmt.Errorf("%s.%s: %w", tgt.file, tgt.structName, err)
		}

		fmt.Fprintf(&buf, "export type %s = {\n%s};\n\n", tgt.tsName, fields)
	}

	return buf.Bytes(), nil
}

const header = `// Code generated by ` + "`go run ./tools/gen-ts-types`" + `. DO NOT EDIT.
//
// Structural mirror of the sqlc-generated + repo wire-facing Go structs
// (issue #365) — see backend/tools/gen-ts-types/main.go for what it does and
// deliberately does not do (enum unions, the full internal/db package).
// Regenerate with ` + "`make backend-gen-ts-types`" + `; CI
// (` + "`make backend-gen-ts-types-check`" + `) fails if this file is stale.

`

func renderStruct(file *ast.File, tgt target) (string, error) {
	structType, err := findStruct(file, tgt.structName)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	for _, field := range structType.Fields.List {
		if len(field.Names) != 1 {
			continue // no embedded/anonymous fields in our targets
		}

		jsonName := jsonFieldName(field)
		if jsonName == "" || jsonName == "-" {
			continue
		}
		if jsonName == defaultSkippedField && !tgt.nullableFields[jsonName] {
			continue
		}

		resolved, err := resolveType(field.Type)
		if err != nil {
			return "", fmt.Errorf("field %s: %w", field.Names[0].Name, err)
		}

		nullable := resolved.nullable || tgt.nullableFields[jsonName]
		tsType := resolved.base
		if nullable {
			tsType += " | null"
		}

		fmt.Fprintf(&buf, "  %s: %s;\n", jsonName, tsType)
	}
	return buf.String(), nil
}

func findStruct(file *ast.File, name string) (*ast.StructType, error) {
	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			continue
		}
		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok || typeSpec.Name.Name != name {
				continue
			}
			structType, ok := typeSpec.Type.(*ast.StructType)
			if !ok {
				return nil, fmt.Errorf("%s is not a struct", name)
			}
			return structType, nil
		}
	}
	return nil, fmt.Errorf("struct %s not found", name)
}

func jsonFieldName(field *ast.Field) string {
	if field.Tag == nil {
		return ""
	}
	tagValue, err := strconv.Unquote(field.Tag.Value)
	if err != nil {
		return ""
	}
	jsonTag := reflect.StructTag(tagValue).Get("json")
	name, _, _ := strings.Cut(jsonTag, ",")
	return name
}

type resolvedType struct {
	base     string // TS base type: "string" | "number" | "boolean"
	nullable bool
}

// resolveType maps a Go field type (as source AST, no type-checking needed)
// to its TS equivalent. It fails loudly on anything it doesn't recognise —
// the point of the allowlist is that a genuinely new shape gets a human's
// attention here rather than silently becoming `any`.
func resolveType(expr ast.Expr) (resolvedType, error) {
	switch t := expr.(type) {
	case *ast.StarExpr:
		inner, err := resolveType(t.X)
		if err != nil {
			return resolvedType{}, err
		}
		inner.nullable = true
		return inner, nil

	case *ast.Ident:
		switch t.Name {
		case "string":
			return resolvedType{base: "string"}, nil
		case "bool":
			return resolvedType{base: "boolean"}, nil
		case "int32", "int64", "int", "float64":
			return resolvedType{base: "number"}, nil
		}
		return resolvedType{}, fmt.Errorf("unmapped type %q — add it to resolveType or exclude the field", t.Name)

	case *ast.SelectorExpr:
		pkgIdent, ok := t.X.(*ast.Ident)
		if !ok {
			return resolvedType{}, fmt.Errorf("unmapped selector expression %v", t)
		}
		switch fmt.Sprintf("%s.%s", pkgIdent.Name, t.Sel.Name) {
		case "uuid.UUID", "decimal.Decimal", "time.Time", "pgtype.Timestamptz":
			return resolvedType{base: "string"}, nil
		default:
			return resolvedType{}, fmt.Errorf("unmapped type %q — add it to resolveType or exclude the field", pkgIdent.Name+"."+t.Sel.Name)
		}
	}
	return resolvedType{}, fmt.Errorf("unmapped expression type %T", expr)
}

func init() {
	// Fail fast with a clear message if run from the wrong working directory
	// (the Makefile always does `cd backend && ...`).
	if _, err := os.Stat(filepath.Join("internal", "db", "models.go")); err != nil {
		fmt.Fprintln(os.Stderr, "gen-ts-types: must be run from backend/ (internal/db/models.go not found)")
		os.Exit(1)
	}
}
