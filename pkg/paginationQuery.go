package pkg

import "fmt"

func PaginationQuery(page int, limit int) string {
	return fmt.Sprintf(`limit %d offset %d`, limit, (page-1)*limit)
}
