package search

import (
	"bytes"
	"regexp"

	P "github.com/cfoust/cy/pkg/io/protocol"
	"github.com/cfoust/cy/pkg/sessions"

	govte "github.com/danielgatis/go-vte"
)

type section struct {
	Index  int
	Offset int
	Start  int
	Bytes  int
	// Whether this section continues into the next without any unprintable
	// characters.
	Continuous bool
}

type Match struct {
	Begin, End Address
	// Whether this Match is unbroken by nonprintable characters. If this
	// is true, it is definitely a match.
	Continuous bool
}

// searcher extracts the printable bytes from its inputs, which it processes as
// though it were a VT100 terminal, and then allows you to search through them
// using regexp.
type searcher struct {
	buffer      bytes.Buffer
	parser      *govte.Parser
	didPrint    bool
	lastPrinted bool
	sections    []section
}

func (s *searcher) Bytes() []byte {
	return s.buffer.Bytes()
}

func getSectionIndex(sections []section, index, offset int, isEnd bool) int {
	current := sections[index]
	for offset >= current.Start+current.Bytes {
		if isEnd && offset == current.Start+current.Bytes {
			return index
		}
		index++
		current = sections[index]
	}

	return index
}

func getAddress(s section, offset int) Address {
	return Address{
		Index:  s.Index,
		Offset: (offset - s.Start) + s.Offset,
	}
}

func (s *searcher) Find(re *regexp.Regexp) (result []Match) {
	matches := re.FindAllIndex(s.buffer.Bytes(), -1)
	if len(matches) == 0 || len(s.sections) == 0 {
		return
	}

	sectionIndex := 0
	for _, match := range matches {
		start := match[0]
		end := match[1]

		startIndex := getSectionIndex(
			s.sections,
			sectionIndex,
			start,
			false,
		)
		endIndex := getSectionIndex(
			s.sections,
			sectionIndex,
			end,
			true,
		)

		isContinuous := true
		for i := startIndex; i < endIndex; i++ {
			if !s.sections[i].Continuous {
				isContinuous = false
				break
			}
		}

		startSection := s.sections[startIndex]
		endSection := s.sections[endIndex]

		result = append(result, Match{
			Begin:      getAddress(startSection, start),
			End:        getAddress(endSection, end),
			Continuous: isContinuous,
		})
		sectionIndex = getSectionIndex(
			s.sections,
			sectionIndex,
			start,
			false,
		)
	}

	return
}

func (s *searcher) parseData(index int, data []byte) {
	isPrinted := false
	count := 0
	for offset, b := range data {
		s.didPrint = false
		s.parser.Advance(b)

		// If this is a direct continuation of the previous section, indicate that
		if offset == 0 && s.didPrint && s.lastPrinted && len(s.sections) > 0 {
			s.sections[len(s.sections)-1].Continuous = true
		}

		s.lastPrinted = s.didPrint

		if s.didPrint == isPrinted {
			count++
			continue
		}

		if count > 0 && isPrinted {
			s.sections = append(s.sections, section{
				Index:  index,
				Offset: offset - count,
				Start:  s.buffer.Len() - count,
				Bytes:  count,
			})
		}

		count = 1
		isPrinted = s.didPrint
	}

	if count > 0 && isPrinted {
		s.sections = append(s.sections, section{
			Index:  index,
			Offset: len(data) - count,
			Start:  s.buffer.Len() - count,
			Bytes:  count,
		})
	}
}

func (s *searcher) Parse(events []sessions.Event) {
	for index, event := range events {
		output, ok := event.Message.(P.OutputMessage)
		if !ok || len(output.Data) == 0 {
			continue
		}
		s.parseData(index, output.Data)
	}
}

func (s *searcher) Print(c rune) {
	s.buffer.Write([]byte{byte(c)})
	s.didPrint = true
}

func (s *searcher) Execute(b byte) {
}

func (s *searcher) Put(b byte) {
}

func (s *searcher) Unhook() {
}

func (s *searcher) Hook(
	params [][]uint16,
	intermediates []byte,
	ignore bool,
	r rune,
) {
}

func (s *searcher) OscDispatch(params [][]byte, bellTerminated bool) {
}

func (s *searcher) CsiDispatch(
	params [][]uint16,
	intermediates []byte,
	ignore bool,
	r rune,
) {
}

func (s *searcher) EscDispatch(intermediates []byte, ignore bool, b byte) {
}

func (s *searcher) SosPmApcDispatch(
	kind govte.SosPmApcKind,
	data []byte,
	bellTerminated bool,
) {
}

func NewSearcher() *searcher {
	p := searcher{}
	p.parser = govte.NewParser(&p)
	return &p
}
