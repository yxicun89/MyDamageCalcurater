package httpapi

import (
	"errors"
	"mime/multipart"
	"net/http"

	"github.com/labstack/echo/v5"

	"example.com/pokecalc/apps/wishlist/api/internal/api"
	"example.com/pokecalc/apps/wishlist/api/internal/item"
	"example.com/pokecalc/apps/wishlist/api/internal/netguard"
	"example.com/pokecalc/apps/wishlist/api/internal/storage"
)

// apiError は Error スキーマで返すエラー。
type apiError struct {
	status int
	code   api.ErrorCode
	msg    string
}

func (e *apiError) Error() string   { return e.msg }
func (e *apiError) StatusCode() int { return e.status }

func badRequest(msg string) error {
	return &apiError{status: http.StatusBadRequest, code: api.ErrorCodeBadRequest, msg: msg}
}

func unprocessable(msg string) error {
	return &apiError{status: http.StatusUnprocessableEntity, code: api.ErrorCodeUnprocessable, msg: msg}
}

func badGateway(msg string) error {
	return &apiError{status: http.StatusBadGateway, code: api.ErrorCodeBadGateway, msg: msg}
}

// classify は err を (status, code, message) にする。ここに載らないものは 500(詳細は応答に出さない)。
func classify(err error) (int, api.ErrorCode, string) {
	var ae *apiError
	if errors.As(err, &ae) {
		return ae.status, ae.code, ae.msg
	}
	var mbe *http.MaxBytesError
	switch {
	case errors.Is(err, item.ErrNotFound), errors.Is(err, storage.ErrNotFound):
		return http.StatusNotFound, api.ErrorCodeNotFound, "not found"
	case errors.Is(err, item.ErrGenreNotFound):
		return http.StatusUnprocessableEntity, api.ErrorCodeUnprocessable, "genre does not exist"
	case errors.Is(err, item.ErrSiteNotFound):
		return http.StatusUnprocessableEntity, api.ErrorCodeUnprocessable, "site does not exist"
	case errors.Is(err, item.ErrDuplicateName):
		return http.StatusUnprocessableEntity, api.ErrorCodeUnprocessable, "name already exists"
	case errors.Is(err, item.ErrInvalid):
		return http.StatusUnprocessableEntity, api.ErrorCodeUnprocessable, err.Error()
	case errors.Is(err, storage.ErrUnsupportedImage):
		return http.StatusUnprocessableEntity, api.ErrorCodeUnprocessable, "unsupported image type (jpeg, png, webp, gif only)"
	case errors.As(err, &mbe):
		return http.StatusUnprocessableEntity, api.ErrorCodeUnprocessable, "request body is too large"
	case errors.Is(err, storage.ErrTooLarge), errors.Is(err, netguard.ErrTooLarge):
		return http.StatusUnprocessableEntity, api.ErrorCodeUnprocessable, "image is too large"
	case errors.Is(err, http.ErrNotMultipart), errors.Is(err, http.ErrMissingBoundary), errors.Is(err, multipart.ErrMessageTooLarge):
		return http.StatusBadRequest, api.ErrorCodeBadRequest, "expected multipart/form-data"
	}
	var sc echo.HTTPStatusCoder
	if errors.As(err, &sc) {
		switch code := sc.StatusCode(); {
		case code == http.StatusNotFound:
			return code, api.ErrorCodeNotFound, "not found"
		case code >= 400 && code < 500:
			return code, api.ErrorCodeBadRequest, requestMessage(err, code)
		}
	}
	return http.StatusInternalServerError, api.ErrorCodeInternal, "internal server error"
}

func requestMessage(err error, code int) string {
	var he *echo.HTTPError
	if errors.As(err, &he) && he.Message != "" {
		return he.Message
	}
	return http.StatusText(code)
}

// handleError は Echo の HTTPErrorHandler。すべてのエラーを Error スキーマで返す。
func (s *server) handleError(c *echo.Context, err error) {
	if r, _ := echo.UnwrapResponse(c.Response()); r != nil && r.Committed {
		return
	}
	status, code, msg := classify(err)
	if status >= 500 && status != http.StatusBadGateway && status != http.StatusNotImplemented {
		s.log.Error("request failed", "method", c.Request().Method, "path", c.Request().URL.Path, "error", err)
	}
	if c.Request().Method == http.MethodHead {
		_ = c.NoContent(status)
		return
	}
	if jerr := c.JSON(status, api.Error{Code: code, Message: msg}); jerr != nil {
		s.log.Error("failed to write error response", "error", jerr)
	}
}
