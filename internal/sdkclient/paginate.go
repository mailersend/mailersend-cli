package sdkclient

import "context"

// PageFetcher fetches a single page of results. Returns the items, whether
// there is a next page, and any error.
type PageFetcher[T any] func(ctx context.Context, page, perPage int) ([]T, bool, error)

// FetchAll fetches all pages up to limit using the given PageFetcher.
// If limit is 0, all pages are fetched. Same pagination logic as the
// old api.Client.GetPaginated.
func FetchAll[T any](ctx context.Context, fetch PageFetcher[T], limit int) ([]T, error) {
	perPage := 25
	if limit > 0 && limit < perPage {
		perPage = limit
	}
	// MailerSend API requires limit >= 10
	if perPage < 10 {
		perPage = 10
	}

	var allItems []T
	page := 1

	for {
		items, hasNext, err := fetch(ctx, page, perPage)
		if err != nil {
			return nil, err
		}

		allItems = append(allItems, items...)

		if limit > 0 && len(allItems) >= limit {
			allItems = allItems[:limit]
			break
		}

		if !hasNext {
			break
		}
		page++
	}

	return allItems, nil
}
