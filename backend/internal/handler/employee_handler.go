package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/slaks37/pos-steca/backend/internal/service"
)

// EmployeeHandler melayani modul karyawan (khusus owner).
type EmployeeHandler struct {
	employees *service.EmployeeService
}

// NewEmployeeHandler membuat handler karyawan.
func NewEmployeeHandler(employees *service.EmployeeService) *EmployeeHandler {
	return &EmployeeHandler{employees: employees}
}

// List mengembalikan daftar karyawan.
func (h *EmployeeHandler) List(c *gin.Context) {
	p, ok := mustPrincipal(c)
	if !ok {
		return
	}
	items, err := h.employees.List(c.Request.Context(), p.TenantID)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

// Create menambahkan karyawan baru.
func (h *EmployeeHandler) Create(c *gin.Context) {
	p, ok := mustPrincipal(c)
	if !ok {
		return
	}
	var in service.EmployeeInput
	if !bindJSON(c, &in) {
		return
	}
	item, err := h.employees.Create(c.Request.Context(), p.TenantID, in)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": item})
}

// Update mengubah data karyawan atau me-reset PIN-nya.
func (h *EmployeeHandler) Update(c *gin.Context) {
	p, ok := mustPrincipal(c)
	if !ok {
		return
	}
	var in service.EmployeeInput
	if !bindJSON(c, &in) {
		return
	}
	item, err := h.employees.Update(c.Request.Context(), p.TenantID, c.Param("id"), in)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": item})
}

// Delete menghapus karyawan.
func (h *EmployeeHandler) Delete(c *gin.Context) {
	p, ok := mustPrincipal(c)
	if !ok {
		return
	}
	if err := h.employees.Delete(c.Request.Context(), p.TenantID, c.Param("id")); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
