package server

import (
	"context"
	"math"

	"github.com/backendArchitect/go-boilerplate/internal/api"
	"github.com/backendArchitect/go-boilerplate/internal/db"
)

// toUser converts a database row to its API shape. Keeping the two separate
// means columns like password_hash can never leak into a response.
func toUser(u db.User) api.User {
	return api.User{ID: u.ID, Name: u.Name, Email: u.Email, CreatedAt: u.CreatedAt, UpdatedAt: u.UpdatedAt}
}

func (s *Server) ListUsers(ctx context.Context, req api.ListUsersRequestObject) (api.ListUsersResponseObject, error) {
	limit, offset := 20, 0
	if p := req.Params.Limit; p != nil {
		limit = *p
	}
	if p := req.Params.Offset; p != nil {
		offset = *p
	}
	if limit < 0 || limit > 100 || offset < 0 || offset > math.MaxInt32 {
		return nil, errorf(api.ErrorCodeInvalidRequest, "limit must be 0 to 100 and offset 0 or more")
	}
	rows, err := s.q.ListUsers(ctx, db.ListUsersParams{Limit: int32(limit), Offset: int32(offset)}) //nolint:gosec // bounds checked above
	if err != nil {
		return nil, err
	}
	users := make([]api.User, len(rows))
	for i, u := range rows {
		users[i] = toUser(u)
	}
	return api.ListUsers200JSONResponse{Users: users}, nil
}

func (s *Server) GetUser(ctx context.Context, req api.GetUserRequestObject) (api.GetUserResponseObject, error) {
	user, err := s.q.GetUser(ctx, req.ID)
	if err != nil {
		return nil, err
	}
	return api.GetUser200JSONResponse(toUser(user)), nil
}

func (s *Server) GetMe(ctx context.Context, _ api.GetMeRequestObject) (api.GetMeResponseObject, error) {
	user, err := s.q.GetUser(ctx, userID(ctx))
	if err != nil {
		return nil, err
	}
	return api.GetMe200JSONResponse(toUser(user)), nil
}

func (s *Server) UpdateMe(ctx context.Context, req api.UpdateMeRequestObject) (api.UpdateMeResponseObject, error) {
	name, email, err := normalizeProfile(req.Body.Name, req.Body.Email)
	if err != nil {
		return nil, err
	}
	user, err := s.q.UpdateUser(ctx, db.UpdateUserParams{ID: userID(ctx), Name: name, Email: email})
	if db.IsUniqueViolation(err) {
		return nil, errEmailTaken
	}
	if err != nil {
		return nil, err
	}
	return api.UpdateMe200JSONResponse(toUser(user)), nil
}

func (s *Server) DeleteMe(ctx context.Context, _ api.DeleteMeRequestObject) (api.DeleteMeResponseObject, error) {
	n, err := s.q.DeleteUser(ctx, userID(ctx))
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, errorf(api.ErrorCodeNotFound, "not found")
	}
	return api.DeleteMe204Response{}, nil
}
