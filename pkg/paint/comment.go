package paint

import "strings"

// CommentStyle describes how a language spells comments.
type CommentStyle struct {
	Line       string // e.g. "//"; empty if the language has no line-comment form
	BlockStart string // e.g. "/*"
	BlockEnd   string // e.g. "*/"
}

var commentStyles = map[string]CommentStyle{
	".go":        {Line: "//", BlockStart: "/*", BlockEnd: "*/"},
	".c":         {Line: "//", BlockStart: "/*", BlockEnd: "*/"},
	".h":         {Line: "//", BlockStart: "/*", BlockEnd: "*/"},
	".cpp":       {Line: "//", BlockStart: "/*", BlockEnd: "*/"},
	".cc":        {Line: "//", BlockStart: "/*", BlockEnd: "*/"},
	".hpp":       {Line: "//", BlockStart: "/*", BlockEnd: "*/"},
	".js":        {Line: "//", BlockStart: "/*", BlockEnd: "*/"},
	".jsx":       {Line: "//", BlockStart: "/*", BlockEnd: "*/"},
	".ts":        {Line: "//", BlockStart: "/*", BlockEnd: "*/"},
	".tsx":       {Line: "//", BlockStart: "/*", BlockEnd: "*/"},
	".rs":        {Line: "//", BlockStart: "/*", BlockEnd: "*/"},
	".java":      {Line: "//", BlockStart: "/*", BlockEnd: "*/"},
	".css":       {BlockStart: "/*", BlockEnd: "*/"},
	".py":        {Line: "#"},
	".rb":        {Line: "#"},
	".sh":        {Line: "#"},
	".bash":      {Line: "#"},
	".yaml":      {Line: "#"},
	".yml":       {Line: "#"},
	".toml":      {Line: "#"},
	".lua":       {Line: "--", BlockStart: "--[[", BlockEnd: "]]"},
	".sql":       {Line: "--"},
	".html":      {BlockStart: "<!--", BlockEnd: "-->"},
	".htm":       {BlockStart: "<!--", BlockEnd: "-->"},
	".xml":       {BlockStart: "<!--", BlockEnd: "-->"},
	".md":        {BlockStart: "<!--", BlockEnd: "-->"},
	".json":      {Line: "//"}, // JSON has no real comment syntax; harmless best-effort default
	".kt":        {Line: "//", BlockStart: "/*", BlockEnd: "*/"},
	".kts":       {Line: "//", BlockStart: "/*", BlockEnd: "*/"},
	".swift":     {Line: "//", BlockStart: "/*", BlockEnd: "*/"},
	".cs":        {Line: "//", BlockStart: "/*", BlockEnd: "*/"},
	".php":       {Line: "//", BlockStart: "/*", BlockEnd: "*/"},
	"dockerfile": {Line: "#"},
}

// StyleForFile returns the comment style for filename, checked first as an
// exact (lowercased) filename -- so extension-less conventions like
// "Dockerfile" resolve correctly -- then by extension, falling back to
// C-style line comments for anything unrecognized.
func StyleForFile(filename string) CommentStyle {
	if style, ok := commentStyles[strings.ToLower(filename)]; ok {
		return style
	}
	if style, ok := commentStyles[extOf(filename)]; ok {
		return style
	}
	return CommentStyle{Line: "//"}
}

func extOf(filename string) string {
	i := strings.LastIndexByte(filename, '.')
	if i < 0 {
		return ""
	}
	return strings.ToLower(filename[i:])
}

// FormatComment wraps art (one string per row, as produced by
// Canvas.Render) into a comment block appropriate for filename's language:
// a line-comment prefix on every row when the language has one (keeps the
// art diffable line-by-line), otherwise a single block comment.
func FormatComment(filename string, art []string) string {
	style := StyleForFile(filename)
	var out []string
	switch {
	case style.Line != "":
		for _, l := range art {
			out = append(out, strings.TrimRight(style.Line+" "+l, " "))
		}
	case style.BlockStart != "":
		out = append(out, style.BlockStart)
		out = append(out, art...)
		out = append(out, style.BlockEnd)
	default:
		for _, l := range art {
			out = append(out, strings.TrimRight("// "+l, " "))
		}
	}
	return strings.Join(out, "\n")
}
