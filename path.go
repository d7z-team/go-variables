package variables

import (
	"fmt"
	"strconv"
	"strings"
)

type PathError struct {
	Op   string
	Path Path
	Err  error
}

func (e *PathError) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Path.IsRoot() {
		return fmt.Sprintf("%s root: %v", e.Op, e.Err)
	}
	return fmt.Sprintf("%s %s: %v", e.Op, e.Path.String(), e.Err)
}

func (e *PathError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

type SegmentKind int

const (
	SegmentKey SegmentKind = iota
	SegmentIndex
)

type Segment struct {
	kind  SegmentKind
	key   string
	index int
}

func Key(key string) Segment {
	return Segment{kind: SegmentKey, key: key}
}

func Index(index int) Segment {
	return Segment{kind: SegmentIndex, index: index}
}

func (s Segment) Kind() SegmentKind {
	return s.kind
}

func (s Segment) Key() string {
	return s.key
}

func (s Segment) Index() int {
	return s.index
}

type Path []Segment

func Root() Path {
	return nil
}

func JoinPath(base Path, segments ...Segment) Path {
	path := make(Path, len(base), len(base)+len(segments))
	copy(path, base)
	return append(path, segments...)
}

func ParsePath(src string) (Path, error) {
	if src == "" {
		return Root(), nil
	}

	var path Path
	i := 0
	needSegment := true
	afterDot := false
	for i < len(src) {
		switch src[i] {
		case '.':
			if needSegment {
				return nil, fmt.Errorf("%w: empty segment in %q", ErrInvalidPath, src)
			}
			needSegment = true
			afterDot = true
			i++
		case '[':
			if afterDot {
				return nil, fmt.Errorf("%w: unexpected bracket after dot in %q", ErrInvalidPath, src)
			}
			seg, next, err := parseBracketSegment(src, i)
			if err != nil {
				return nil, err
			}
			path = append(path, seg)
			i = next
			needSegment = false
			afterDot = false
		default:
			if !needSegment {
				return nil, fmt.Errorf("%w: expected delimiter at byte %d in %q", ErrInvalidPath, i, src)
			}
			start := i
			for i < len(src) && src[i] != '.' && src[i] != '[' {
				i++
			}
			if start == i {
				return nil, fmt.Errorf("%w: empty segment in %q", ErrInvalidPath, src)
			}
			path = append(path, Key(src[start:i]))
			needSegment = false
			afterDot = false
		}
	}
	if needSegment {
		return nil, fmt.Errorf("%w: trailing dot in %q", ErrInvalidPath, src)
	}
	return path, nil
}

func MustPath(src string) Path {
	path, err := ParsePath(src)
	if err != nil {
		panic(err)
	}
	return path
}

func (p Path) IsRoot() bool {
	return len(p) == 0
}

func (p Path) Child(segments ...Segment) Path {
	return JoinPath(p, segments...)
}

func (p Path) Parent() (Path, Segment, bool) {
	if len(p) == 0 {
		return nil, Segment{}, false
	}
	parent := make(Path, len(p)-1)
	copy(parent, p[:len(p)-1])
	return parent, p[len(p)-1], true
}

func (p Path) Segments() []Segment {
	segments := make([]Segment, len(p))
	copy(segments, p)
	return segments
}

func (p Path) String() string {
	if len(p) == 0 {
		return ""
	}
	var b strings.Builder
	for i, seg := range p {
		switch seg.kind {
		case SegmentKey:
			if isBareKey(seg.key) {
				if i > 0 {
					b.WriteByte('.')
				}
				b.WriteString(seg.key)
			} else {
				b.WriteString("[")
				b.WriteString(strconv.Quote(seg.key))
				b.WriteString("]")
			}
		case SegmentIndex:
			b.WriteByte('[')
			b.WriteString(strconv.Itoa(seg.index))
			b.WriteByte(']')
		}
	}
	return b.String()
}

func parseBracketSegment(src string, start int) (Segment, int, error) {
	if start+1 >= len(src) {
		return Segment{}, 0, fmt.Errorf("%w: unclosed bracket in %q", ErrInvalidPath, src)
	}
	if src[start+1] == '"' || src[start+1] == '\'' {
		key, next, err := parseQuotedKey(src, start+1)
		if err != nil {
			return Segment{}, 0, err
		}
		if next >= len(src) || src[next] != ']' {
			return Segment{}, 0, fmt.Errorf("%w: unclosed quoted key in %q", ErrInvalidPath, src)
		}
		return Key(key), next + 1, nil
	}

	end := start + 1
	for end < len(src) && src[end] != ']' {
		end++
	}
	if end >= len(src) {
		return Segment{}, 0, fmt.Errorf("%w: unclosed bracket in %q", ErrInvalidPath, src)
	}
	raw := strings.TrimSpace(src[start+1 : end])
	if raw == "" {
		return Segment{}, 0, fmt.Errorf("%w: empty index in %q", ErrInvalidPath, src)
	}
	index, err := strconv.Atoi(raw)
	if err != nil || index < 0 {
		return Segment{}, 0, fmt.Errorf("%w: invalid index %q", ErrInvalidPath, raw)
	}
	return Index(index), end + 1, nil
}

func parseQuotedKey(src string, start int) (string, int, error) {
	quote := src[start]
	if quote == '"' {
		for i := start + 1; i < len(src); i++ {
			if src[i] == '\\' {
				i++
				if i >= len(src) {
					return "", 0, fmt.Errorf("%w: dangling escape in %q", ErrInvalidPath, src)
				}
				continue
			}
			if src[i] == quote {
				key, err := strconv.Unquote(src[start : i+1])
				if err != nil {
					return "", 0, fmt.Errorf("%w: invalid quoted key in %q", ErrInvalidPath, src)
				}
				return key, i + 1, nil
			}
		}
		return "", 0, fmt.Errorf("%w: unclosed quote in %q", ErrInvalidPath, src)
	}

	var b strings.Builder
	for i := start + 1; i < len(src); i++ {
		if src[i] == quote {
			return b.String(), i + 1, nil
		}
		if src[i] == '\\' {
			i++
			if i >= len(src) {
				return "", 0, fmt.Errorf("%w: dangling escape in %q", ErrInvalidPath, src)
			}
			switch src[i] {
			case 'n':
				b.WriteByte('\n')
			case 'r':
				b.WriteByte('\r')
			case 't':
				b.WriteByte('\t')
			default:
				b.WriteByte(src[i])
			}
			continue
		}
		b.WriteByte(src[i])
	}
	return "", 0, fmt.Errorf("%w: unclosed quote in %q", ErrInvalidPath, src)
}

func isBareKey(key string) bool {
	if key == "" {
		return false
	}
	for _, r := range key {
		if (r >= 'a' && r <= 'z') ||
			(r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') ||
			r == '_' || r == '-' || r == '@' || r == '#' {
			continue
		}
		return false
	}
	return true
}
