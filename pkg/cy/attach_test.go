package cy

import (
	"fmt"
	"testing"
	"time"

	"github.com/cfoust/cy/pkg/cy/cmd"
	"github.com/cfoust/cy/pkg/geom"
	T "github.com/cfoust/cy/pkg/mux/screen/tree"
	"github.com/cfoust/cy/pkg/mux/stream"
	"github.com/cfoust/cy/pkg/params"

	"github.com/stretchr/testify/require"
)

// newPane creates a standalone shell pane under the tree root.
func newPane(t *testing.T, server *Cy) *T.Pane {
	group := server.tree.Root().NewGroup()
	command, err := cmd.New(
		server.Ctx(),
		stream.CmdOptions{
			Command: "/bin/bash",
		},
		params.New(),
		server.timeBinds,
		server.copyBinds,
	)
	require.NoError(t, err)
	return group.NewPane(
		server.Ctx(),
		command,
	)
}

// TestHookAttach verifies that attaching to a different node fires hook/attach
// with the previous and new node IDs and the path to the attached view.
func TestHookAttach(t *testing.T) {
	server, create := setup(t)
	client := create(geom.DEFAULT_SIZE)

	// Record the arguments hook/attach receives into client parameters. The
	// layout is validated inside Janet so the assertion does not depend on
	// how it marshals back into Go: :layout-ok checks that the attached
	// view within the passed layout has the new node's id, and that its
	// path (derived with layout/attach-path) is the expected [:b].
	require.NoError(t, client.execute(`
(defn hook/attach [prev id layout]
  (param/set :client :prev-attach prev)
  (param/set :client :last-attach id)
  (def path (layout/attach-path layout))
  (param/set :client :layout-ok
             (and (deep= path @[:b])
                  (= id ((layout/path layout path) :id)))))
`))

	first := newPane(t, server)
	require.NoError(t, client.Attach(first))

	require.Eventually(t, func() bool {
		value, ok := client.Get("last-attach")
		return ok && value != nil &&
			fmt.Sprint(value) == fmt.Sprint(first.Id())
	}, 5*time.Second, 10*time.Millisecond,
		"hook/attach did not fire for the first attach")

	// Attach to a second pane via a split layout, with the attached view in
	// the :b branch, so the path to the attached view is [:b].
	second := newPane(t, server)
	require.NoError(t, client.execute(fmt.Sprintf(`
(layout/set (layout/split
  (layout/view :id %d)
  (layout/view :id %d :attached true)))
`, first.Id(), second.Id())))

	require.Eventually(t, func() bool {
		prev, prevOK := client.Get("prev-attach")
		last, lastOK := client.Get("last-attach")
		layoutOK, layoutPresent := client.Get("layout-ok")
		if !prevOK || !lastOK || !layoutPresent ||
			prev == nil || last == nil {
			return false
		}
		return fmt.Sprint(prev) == fmt.Sprint(first.Id()) &&
			fmt.Sprint(last) == fmt.Sprint(second.Id()) &&
			fmt.Sprint(layoutOK) == "true"
	}, 5*time.Second, 10*time.Millisecond,
		"hook/attach did not fire with the expected IDs and layout")
}
