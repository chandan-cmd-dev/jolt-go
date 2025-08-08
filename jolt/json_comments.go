package jolt

// StripJSONComments removes // line and /* block */ comments from JSON bytes while
// preserving string contents.
func StripJSONComments(in []byte) []byte {
    const (
        sNorm = iota
        sStr
        sStrEsc
        sSlash
        sLine
        sBlock
        sBlockStar
    )
    out := make([]byte, 0, len(in))
    state := sNorm
    for i := 0; i < len(in); i++ {
        c := in[i]
        switch state {
        case sNorm:
            if c == '"' {
                out = append(out, c)
                state = sStr
            } else if c == '/' {
                state = sSlash
            } else {
                out = append(out, c)
            }
        case sSlash:
            if c == '/' {
                state = sLine
            } else if c == '*' {
                state = sBlock
            } else {
                out = append(out, '/')
                out = append(out, c)
                state = sNorm
            }
        case sLine:
            if c == '\n' || c == '\r' {
                out = append(out, c)
                state = sNorm
            }
        case sBlock:
            if c == '*' {
                state = sBlockStar
            }
        case sBlockStar:
            if c == '/' {
                state = sNorm
            } else if c != '*' {
                state = sBlock
            }
        case sStr:
            if c == '\\' {
                out = append(out, c)
                state = sStrEsc
            } else if c == '"' {
                out = append(out, c)
                state = sNorm
            } else {
                out = append(out, c)
            }
        case sStrEsc:
            out = append(out, c)
            state = sStr
        }
    }
    if state == sSlash {
        out = append(out, '/')
    }
    return out
}
