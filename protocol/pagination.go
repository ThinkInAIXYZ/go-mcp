package protocol

import (
	"encoding/base64"
	"sort"
)

// Cursor is an opaque token used to represent a cursor for pagination.
type Cursor string

type Named interface {
	GetName() string
}

func PaginationLimit[T Named](allElements []T, cursor Cursor, limit int) ([]T, Cursor, error) {
	sort.Slice(allElements, func(i, j int) bool {
		return allElements[i].GetName() < allElements[j].GetName()
	})
	startPos := 0
	if cursor != "" {
		c, err := base64.StdEncoding.DecodeString(string(cursor))
		if err != nil {
			return nil, "", err
		}
		cString := string(c)
		startPos = sort.Search(len(allElements), func(i int) bool {
			nc := allElements[i].GetName()
			return nc > cString
		})
	}
	if limit >= len(allElements)-startPos {
		return allElements[startPos:], "", nil
	}
	endPos := startPos + limit
	lastName := allElements[endPos-1].GetName()
	nextCursor := base64.StdEncoding.EncodeToString([]byte(lastName))
	return allElements[startPos:endPos], Cursor(nextCursor), nil
}

// PaginatedRequest represents a request that supports pagination
type PaginatedRequest struct {
	Cursor Cursor `json:"cursor,omitempty"`
}

// PaginatedResult represents a response that supports pagination
type PaginatedResult struct {
	NextCursor Cursor `json:"nextCursor,omitempty"`
}
