package command

// TextSplitter buffers incomplete text and emits LF, CR, and CRLF-delimited lines.
type TextSplitter struct {
	buffer     []byte
	bufferedCR bool
	consume    func(string, bool)
}

func NewTextSplitter(consume func(string, bool)) *TextSplitter {
	return &TextSplitter{consume: consume}
}

func (s *TextSplitter) Process(input []byte) {
	offset := 0
	for offset < len(input) {
		next := offset
		for next < len(input) && input[next] != '\n' && input[next] != '\r' {
			next++
		}
		if next == len(input) {
			if s.bufferedCR {
				s.send()
			}
			s.buffer = append(s.buffer, input[offset:]...)
			return
		}
		isCR := input[next] == '\r'
		if isCR && next+1 == len(input) {
			if s.bufferedCR {
				s.send()
			}
			s.buffer = append(s.buffer, input[offset:next]...)
			s.bufferedCR = true
			return
		}
		if s.bufferedCR {
			if input[offset] == '\n' {
				s.bufferedCR = false
			} else {
				s.send()
			}
		}
		crlf := isCR && input[next+1] == '\n'
		crLine := isCR && !crlf
		if len(s.buffer) == 0 {
			s.consume(string(input[offset:next]), crLine)
		} else {
			s.bufferedCR = crLine
			s.buffer = append(s.buffer, input[offset:next]...)
			s.send()
		}
		if crlf {
			offset = next + 2
		} else {
			offset = next + 1
		}
	}
}

func (s *TextSplitter) send() {
	s.consume(string(s.buffer), s.bufferedCR)
	s.buffer = s.buffer[:0]
	s.bufferedCR = false
}

func (s *TextSplitter) Flush() {
	if len(s.buffer) > 0 || s.bufferedCR {
		s.send()
	}
}
