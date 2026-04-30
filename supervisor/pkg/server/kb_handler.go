package server

import (
	"github.com/gofiber/fiber/v2"
	"github.com/quarkloop/supervisor/pkg/api"
	"github.com/quarkloop/supervisor/pkg/kb"
)

// openKB opens the KB store for the given space name.
func (s *Server) openKB(name string) (kb.Store, error) {
	return s.store.KB(name)
}

func (s *Server) handleKBGet(c *fiber.Ctx) error {
	name := c.Params("name")
	store, err := s.openKB(name)
	if err != nil {
		return s.writeSpaceError(c, name, err)
	}
	defer store.Close()

	val, err := store.Get(c.Params("namespace"), c.Params("key"))
	if err != nil {
		return writeError(c, fiber.StatusNotFound, err.Error())
	}
	return writeJSON(c, fiber.StatusOK, api.KBValueResponse{Value: val})
}

func (s *Server) handleKBSet(c *fiber.Ctx) error {
	name := c.Params("name")
	store, err := s.openKB(name)
	if err != nil {
		return s.writeSpaceError(c, name, err)
	}
	defer store.Close()

	var req api.KBSetRequest
	if err := c.BodyParser(&req); err != nil {
		return writeError(c, fiber.StatusBadRequest, "invalid JSON body")
	}
	if err := store.Set(c.Params("namespace"), c.Params("key"), req.Value); err != nil {
		return writeError(c, fiber.StatusInternalServerError, err.Error())
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (s *Server) handleKBDelete(c *fiber.Ctx) error {
	name := c.Params("name")
	store, err := s.openKB(name)
	if err != nil {
		return s.writeSpaceError(c, name, err)
	}
	defer store.Close()

	if err := store.Delete(c.Params("namespace"), c.Params("key")); err != nil {
		return writeError(c, fiber.StatusNotFound, err.Error())
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (s *Server) handleKBList(c *fiber.Ctx) error {
	name := c.Params("name")
	store, err := s.openKB(name)
	if err != nil {
		return s.writeSpaceError(c, name, err)
	}
	defer store.Close()

	keys, err := store.List(c.Params("namespace"))
	if err != nil {
		return writeError(c, fiber.StatusInternalServerError, err.Error())
	}
	return writeJSON(c, fiber.StatusOK, api.KBListResponse{Keys: keys})
}
