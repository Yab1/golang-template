package query

// ListResult is the shared store return for paginated lists.
// Not the HTTP envelope — handlers map Items + meta into httpx.JSONList.
type ListResult[T any] struct {
	Items      []T
	Total      int64
	NextCursor string
}
