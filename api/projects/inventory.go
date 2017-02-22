package projects

import (
	"database/sql"

	database "github.com/ansible-semaphore/semaphore/db"
	"github.com/ansible-semaphore/semaphore/models"
	"github.com/ansible-semaphore/semaphore/util"
	"github.com/gin-gonic/gin"
	"github.com/masterminds/squirrel"
)

func InventoryMiddleware(w http.ResponseWriter, r *http.Request) {
	project := c.MustGet("project").(models.Project)
	inventoryID, err := util.GetIntParam("inventory_id", c)
	if err != nil {
		return
	}

	query, args, _ := squirrel.Select("*").
		From("project__inventory").
		Where("project_id=?", project.ID).
		Where("id=?", inventoryID).
		ToSql()

	var inventory models.Inventory
	if err := database.Mysql.SelectOne(&inventory, query, args...); err != nil {
		if err == sql.ErrNoRows {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		panic(err)
	}

	c.Set("inventory", inventory)
	c.Next()
}

func GetInventory(w http.ResponseWriter, r *http.Request) {
	project := c.MustGet("project").(models.Project)
	var inv []models.Inventory

	query, args, _ := squirrel.Select("*").
		From("project__inventory").
		Where("project_id=?", project.ID).
		ToSql()

	if _, err := database.Mysql.Select(&inv, query, args...); err != nil {
		panic(err)
	}

	c.JSON(200, inv)
}

func AddInventory(w http.ResponseWriter, r *http.Request) {
	project := c.MustGet("project").(models.Project)
	var inventory struct {
		Name      string `json:"name" binding:"required"`
		KeyID     *int   `json:"key_id"`
		SshKeyID  int    `json:"ssh_key_id"`
		Type      string `json:"type"`
		Inventory string `json:"inventory"`
	}

	if err := c.Bind(&inventory); err != nil {
		return
	}

	switch inventory.Type {
	case "static", "aws", "do", "gcloud":
		break
	default:
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	res, err := database.Mysql.Exec("insert into project__inventory set project_id=?, name=?, type=?, key_id=?, ssh_key_id=?, inventory=?", project.ID, inventory.Name, inventory.Type, inventory.KeyID, inventory.SshKeyID, inventory.Inventory)
	if err != nil {
		panic(err)
	}

	insertID, _ := res.LastInsertId()
	insertIDInt := int(insertID)
	objType := "inventory"

	desc := "Inventory " + inventory.Name + " created"
	if err := (models.Event{
		ProjectID:   &project.ID,
		ObjectType:  &objType,
		ObjectID:    &insertIDInt,
		Description: &desc,
	}.Insert()); err != nil {
		panic(err)
	}

	w.WriteHeader(http.StatusNoContent)
}

func UpdateInventory(w http.ResponseWriter, r *http.Request) {
	oldInventory := c.MustGet("inventory").(models.Inventory)

	var inventory struct {
		Name      string `json:"name" binding:"required"`
		KeyID     *int   `json:"key_id"`
		SshKeyID  int    `json:"ssh_key_id"`
		Type      string `json:"type"`
		Inventory string `json:"inventory"`
	}

	if err := c.Bind(&inventory); err != nil {
		return
	}

	switch inventory.Type {
	case "static", "aws", "do", "gcloud":
		break
	default:
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if _, err := database.Mysql.Exec("update project__inventory set name=?, type=?, key_id=?, ssh_key_id=?, inventory=? where id=?", inventory.Name, inventory.Type, inventory.KeyID, inventory.SshKeyID, inventory.Inventory, oldInventory.ID); err != nil {
		panic(err)
	}

	desc := "Inventory " + inventory.Name + " updated"
	objType := "inventory"
	if err := (models.Event{
		ProjectID:   &oldInventory.ProjectID,
		Description: &desc,
		ObjectID:    &oldInventory.ID,
		ObjectType:  &objType,
	}.Insert()); err != nil {
		panic(err)
	}

	w.WriteHeader(http.StatusNoContent)
}

func RemoveInventory(w http.ResponseWriter, r *http.Request) {
	inventory := c.MustGet("inventory").(models.Inventory)

	templatesC, err := database.Mysql.SelectInt("select count(1) from project__template where project_id=? and inventory_id=?", inventory.ProjectID, inventory.ID)
	if err != nil {
		panic(err)
	}

	if templatesC > 0 {
		if len(c.Query("setRemoved")) == 0 {
			c.JSON(400, map[string]interface{}{
				"error": "Inventory is in use by one or more templates",
				"inUse": true,
			})

			return
		}

		if _, err := database.Mysql.Exec("update project__inventory set removed=1 where id=?", inventory.ID); err != nil {
			panic(err)
		}

		w.WriteHeader(http.StatusNoContent)
		return
	}

	if _, err := database.Mysql.Exec("delete from project__inventory where id=?", inventory.ID); err != nil {
		panic(err)
	}

	desc := "Inventory " + inventory.Name + " deleted"
	if err := (models.Event{
		ProjectID:   &inventory.ProjectID,
		Description: &desc,
	}.Insert()); err != nil {
		panic(err)
	}

	w.WriteHeader(http.StatusNoContent)
}
