package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStatusTransitions(t *testing.T) {
	t.Parallel()
	tests := []struct {
		from, to Status
		allowed  bool
	}{
		{StatusTODO, StatusInProgress, true},
		{StatusTODO, StatusCancelled, true},
		{StatusTODO, StatusDone, false},
		{StatusInProgress, StatusInReview, true},
		{StatusInProgress, StatusDone, true},
		{StatusInReview, StatusInProgress, true},
		{StatusInReview, StatusDone, true},
		{StatusInReview, StatusTODO, false},
		{StatusDone, StatusInProgress, true},
		{StatusDone, StatusCancelled, false},
		{StatusCancelled, StatusTODO, true},
		{StatusCancelled, StatusDone, false},
		{StatusTODO, StatusTODO, true},
	}
	for _, tt := range tests {
		t.Run(string(tt.from)+"_to_"+string(tt.to), func(t *testing.T) {
			require.Equal(t, tt.allowed, tt.from.CanTransitionTo(tt.to))
		})
	}
}

func TestStatusAndPriorityValidation(t *testing.T) {
	t.Parallel()
	require.True(t, StatusInReview.Valid())
	require.False(t, Status("UNKNOWN").Valid())
	require.True(t, PriorityUrgent.Valid())
	require.False(t, Priority("BLOCKER").Valid())
}
