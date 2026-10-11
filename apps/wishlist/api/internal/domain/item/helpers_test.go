package item_test

// strp は文字列へのポインタを返す(テストの組み立て用。app/items のテストにも同名がある)。
func strp(s string) *string { return &s }
