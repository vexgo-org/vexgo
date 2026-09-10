/**
 * `go` wraps a Go template expression so it can be emitted as literal text
 * from TSX. React renders the string verbatim (double braces are not JSX
 * syntax), so the built HTML page carries e.g. `{{range .Posts}}` which the
 * backend then executes as a Go template.
 *
 * Rule: expressions placed inside an HTML *attribute* must not contain
 * double quotes (React escapes them to &quot; which breaks Go template
 * parsing). Use the backend helper funcs (userURL, categoryURL, ...) there.
 */
export function go(expr: string): string {
  return `{{${expr}}}`;
}
