package variables

import (
	"bufio"
	"fmt"
	"strings"
)

func decodeProperties(src string, infer bool) (any, error) {
	root := map[string]any{}
	scanner := bufio.NewScanner(strings.NewReader(src))
	scanner.Buffer(make([]byte, 1024), 1024*1024)
	var logical strings.Builder
	lineNumber := 0

	for scanner.Scan() {
		lineNumber++
		line := strings.TrimRight(scanner.Text(), "\r")
		if logical.Len() > 0 {
			line = strings.TrimLeft(line, " \t\f")
		}
		if continues(line) {
			logical.WriteString(line[:len(line)-1])
			continue
		}
		logical.WriteString(line)
		entry := strings.TrimLeft(logical.String(), " \t\f")
		logical.Reset()
		if entry == "" || entry[0] == '#' || entry[0] == '!' {
			continue
		}

		key, value, err := splitProperty(entry)
		if err != nil {
			return nil, fmt.Errorf("properties line %d: %w", lineNumber, err)
		}
		path, err := ParsePath(unescapeProperty(key))
		if err != nil {
			return nil, fmt.Errorf("properties line %d: %w", lineNumber, err)
		}
		decoded := any(unescapeProperty(value))
		if infer {
			decoded = inferScalar(decoded.(string))
		}
		if err := setAtAny(root, path, decoded); err != nil {
			return nil, fmt.Errorf("properties line %d: %w", lineNumber, err)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if logical.Len() > 0 {
		return nil, fmt.Errorf("properties: dangling continuation")
	}
	return root, nil
}

func setAtAny(root map[string]any, path Path, value any) error {
	if path.IsRoot() {
		return ErrInvalidPath
	}
	var node any = root
	return setAtAnyNode(&node, path, value)
}

func setAtAnyNode(node *any, path Path, value any) error {
	seg := path[0]
	last := len(path) == 1
	switch seg.kind {
	case SegmentKey:
		current, ok := (*node).(map[string]any)
		if !ok {
			return ErrTypeConflict
		}
		if last {
			current[seg.key] = value
			return nil
		}
		child, ok := current[seg.key]
		if !ok || child == nil {
			child = newAnyContainer(path[1])
		}
		if err := setAtAnyNode(&child, path[1:], value); err != nil {
			return err
		}
		current[seg.key] = child
		return nil
	case SegmentIndex:
		current, ok := (*node).([]any)
		if !ok {
			return ErrTypeConflict
		}
		if seg.index < 0 {
			return ErrIndexOutOfRange
		}
		for len(current) <= seg.index {
			current = append(current, nil)
		}
		if last {
			current[seg.index] = value
			*node = current
			return nil
		}
		child := current[seg.index]
		if child == nil {
			child = newAnyContainer(path[1])
		}
		if err := setAtAnyNode(&child, path[1:], value); err != nil {
			return err
		}
		current[seg.index] = child
		*node = current
		return nil
	default:
		return ErrInvalidPath
	}
}

func newAnyContainer(next Segment) any {
	if next.kind == SegmentIndex {
		return []any{}
	}
	return map[string]any{}
}

func splitProperty(line string) (string, string, error) {
	sep := -1
	escaped := false
	for i, r := range line {
		if escaped {
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		if r == '=' || r == ':' || r == ' ' || r == '\t' || r == '\f' {
			sep = i
			break
		}
	}
	if sep < 0 {
		return line, "", nil
	}
	key := strings.TrimRight(line[:sep], " \t\f")
	rest := strings.TrimLeft(line[sep:], " \t\f")
	if rest != "" && (rest[0] == '=' || rest[0] == ':') {
		rest = strings.TrimLeft(rest[1:], " \t\f")
	}
	if key == "" {
		return "", "", ErrInvalidPath
	}
	return key, rest, nil
}

func continues(line string) bool {
	count := 0
	for i := len(line) - 1; i >= 0 && line[i] == '\\'; i-- {
		count++
	}
	return count%2 == 1
}

func unescapeProperty(src string) string {
	var b strings.Builder
	for i := 0; i < len(src); i++ {
		if src[i] != '\\' || i+1 >= len(src) {
			b.WriteByte(src[i])
			continue
		}
		i++
		switch src[i] {
		case 'n':
			b.WriteByte('\n')
		case 'r':
			b.WriteByte('\r')
		case 't':
			b.WriteByte('\t')
		case 'f':
			b.WriteByte('\f')
		default:
			b.WriteByte(src[i])
		}
	}
	return b.String()
}
