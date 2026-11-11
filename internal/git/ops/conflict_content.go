package ops

import (
	"context"
	"fmt"
)

// ConflictContent uses the same side convention as AcceptOurs/AcceptTheirs.
type ConflictContent struct {
	Base, Left, Right     string
	LeftLabel, RightLabel string
}

func (o *Service) ConflictContent(ctx context.Context, conflict Conflict) (ConflictContent, error) {
	content := ConflictContent{LeftLabel: "Yours", RightLabel: "Theirs"}
	values := []*string{&content.Base, &content.Left, &content.Right}
	for i, exists := range []bool{conflict.Base, conflict.Ours, conflict.Theirs} {
		if !exists {
			continue
		}
		res, err := o.run(ctx, "show", fmt.Sprintf(":%d:%s", i+1, conflict.Path))
		if err != nil {
			return content, err
		}
		*values[i] = string(res.Stdout)
	}
	if err := ctx.Err(); err != nil {
		return content, err
	}
	if o.isRebasing(ctx) {
		content.Left, content.Right = content.Right, content.Left
		content.LeftLabel = "Yours (rebasing branch)"
		content.RightLabel = "Theirs (upstream)"
	}
	return content, nil
}
