package dto

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUpdateRequestDistinguishesOmittedAndNullFields(t *testing.T) {
	t.Parallel()
	var omitted UpdateRequest
	require.NoError(t, json.Unmarshal([]byte(`{"version":1}`), &omitted))
	require.False(t, omitted.AssigneeID.Set)
	require.False(t, omitted.DueAt.Set)

	var cleared UpdateRequest
	require.NoError(t, json.Unmarshal([]byte(`{"version":1,"assigneeId":null,"dueAt":null}`), &cleared))
	require.True(t, cleared.AssigneeID.Set)
	require.Nil(t, cleared.AssigneeID.Value)
	require.True(t, cleared.DueAt.Set)
	require.Nil(t, cleared.DueAt.Value)
}
