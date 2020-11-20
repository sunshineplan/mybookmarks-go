package main

import (
	"fmt"
	"log"
	"strconv"

	"github.com/gin-gonic/contrib/sessions"
	"github.com/gin-gonic/gin"
)

type bookmark struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	URL      string `json:"url"`
	Category string `json:"category"`
}

func getBookmark(c *gin.Context) {
	var r struct{ Category, Start int }
	if err := c.BindJSON(&r); err != nil {
		c.String(400, "")
		return
	}

	db, err := getDB()
	if err != nil {
		log.Println("Failed to connect to database:", err)
		c.String(503, "")
		return
	}
	defer db.Close()

	session := sessions.Default(c)
	userID := session.Get("user_id")

	stmt := "SELECT %s FROM bookmarks WHERE"

	var args []interface{}
	switch r.Category {
	case -1:
		stmt += " user_id = ?"
		args = append(args, userID)
	case 0:
		stmt += " category_id = 0 AND user_id = ?"
		args = append(args, userID)
	default:
		stmt += " category_id = ? AND user_id = ?"
		args = append(args, r.Category)
		args = append(args, userID)
	}

	limit := fmt.Sprintf(" LIMIT %d, 30", r.Start)
	rows, err := db.Query(fmt.Sprintf(stmt+limit, "bookmark_id, bookmark, url, category"), args...)
	if err != nil {
		log.Println("Failed to get bookmarks:", err)
		c.String(500, "")
		return
	}
	defer rows.Close()
	bookmarks := []bookmark{}
	for rows.Next() {
		var bookmark bookmark
		var categoryByte []byte
		if err := rows.Scan(&bookmark.ID, &bookmark.Name, &bookmark.URL, &categoryByte); err != nil {
			log.Println("Failed to scan bookmarks:", err)
			c.String(500, "")
			return
		}
		bookmark.Category = string(categoryByte)
		bookmarks = append(bookmarks, bookmark)
	}
	c.JSON(200, bookmarks)
}

func addBookmark(c *gin.Context) {
	db, err := getDB()
	if err != nil {
		log.Println("Failed to connect to database:", err)
		c.String(503, "")
		return
	}
	defer db.Close()

	session := sessions.Default(c)
	userID := session.Get("user_id")

	var bookmark bookmark
	if err := c.BindJSON(&bookmark); err != nil {
		c.String(400, "")
		return
	}

	bc := make(chan error, 3)
	var categoryID int
	var exist1, exist2 string
	go func() {
		var err error
		categoryID, err = getCategoryID(bookmark.Category, userID.(int), db)
		bc <- err
	}()
	go func() {
		db.QueryRow("SELECT id FROM bookmark WHERE bookmark = ? AND user_id = ?",
			bookmark.Name, userID).Scan(&exist1)
		bc <- nil
	}()
	go func() {
		db.QueryRow("SELECT id FROM bookmark WHERE url = ? AND user_id = ?",
			bookmark.URL, userID).Scan(&exist2)
		bc <- nil
	}()
	for i := 0; i < 3; i++ {
		if err := <-bc; err != nil {
			log.Println("Failed to get category id:", err)
			c.String(500, "")
			return
		}
	}

	var message string
	var errorCode int
	switch {
	case bookmark.Name == "":
		message = "Bookmark name is empty."
		errorCode = 1
	case exist1 != "":
		message = fmt.Sprintf("Bookmark name %s is already existed.", bookmark.Name)
		errorCode = 1
	case exist2 != "":
		message = fmt.Sprintf("Bookmark url %s is already existed.", bookmark.URL)
		errorCode = 2
	case categoryID == -1:
		message = "Category name exceeded length limit."
		errorCode = 3
	default:
		if _, err := db.Exec("INSERT INTO bookmark (bookmark, url, user_id, category_id) VALUES (?, ?, ?, ?)",
			bookmark.Name, bookmark.URL, userID, categoryID); err != nil {
			log.Println("Failed to add bookmark:", err)
			c.String(500, "")
			return
		}
		c.JSON(200, gin.H{"status": 1})
		return
	}
	c.JSON(200, gin.H{"status": 0, "message": message, "error": errorCode})
}

func editBookmark(c *gin.Context) {
	db, err := getDB()
	if err != nil {
		log.Println("Failed to connect to database:", err)
		c.String(503, "")
		return
	}
	defer db.Close()

	session := sessions.Default(c)
	userID := session.Get("user_id")

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		log.Println("Failed to get id param:", err)
		c.String(400, "")
		return
	}
	var new bookmark
	if err := c.BindJSON(&new); err != nil {
		c.String(400, "")
		return
	}

	bc := make(chan error, 4)
	var old bookmark
	var categoryID int
	var exist1, exist2 string
	go func() {
		var oldCategory []byte
		err := db.QueryRow("SELECT bookmark, url, category FROM bookmarks WHERE bookmark_id = ? AND user_id = ?",
			id, userID).Scan(&old.Name, &old.URL, &oldCategory)
		old.Category = string(oldCategory)
		bc <- err
	}()
	go func() {
		var err error
		categoryID, err = getCategoryID(new.Category, userID.(int), db)
		bc <- err
	}()
	go func() {
		db.QueryRow("SELECT id FROM bookmark WHERE bookmark = ? AND id != ? AND user_id = ?",
			new.Name, id, userID).Scan(&exist1)
		bc <- nil
	}()
	go func() {
		db.QueryRow("SELECT id FROM bookmark WHERE url = ? AND id != ? AND user_id = ?",
			new.URL, id, userID).Scan(&exist2)
		bc <- nil
	}()
	for i := 0; i < 4; i++ {
		if err := <-bc; err != nil {
			log.Println(err)
			c.String(500, "")
			return
		}
	}

	var message string
	var errorCode int
	switch {
	case new.Name == "":
		message = "Bookmark name is empty."
		errorCode = 1
	case old == new:
		message = "New bookmark is same as old bookmark."
	case exist1 != "":
		message = fmt.Sprintf("Bookmark name %s is already existed.", new.Name)
		errorCode = 1
	case exist2 != "":
		message = fmt.Sprintf("Bookmark url %s is already existed.", new.URL)
		errorCode = 2
	case categoryID == -1:
		message = "Category name exceeded length limit."
		errorCode = 3
	default:
		if _, err := db.Exec("UPDATE bookmark SET bookmark = ?, url = ?, category_id = ? WHERE id = ? AND user_id = ?",
			new.Name, new.URL, categoryID, id, userID); err != nil {
			log.Println("Failed to edit bookmark:", err)
			c.String(500, "")
			return
		}
		c.JSON(200, gin.H{"status": 1})
		return
	}
	c.JSON(200, gin.H{"status": 0, "message": message, "error": errorCode})
}

func deleteBookmark(c *gin.Context) {
	db, err := getDB()
	if err != nil {
		log.Println("Failed to connect to database:", err)
		c.String(503, "")
		return
	}
	defer db.Close()

	session := sessions.Default(c)
	userID := session.Get("user_id")

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		log.Println("Failed to get id param:", err)
		c.String(400, "")
		return
	}

	if _, err := db.Exec("DELETE FROM bookmark WHERE id = ? and user_id = ?", id, userID); err != nil {
		log.Println("Failed to delete bookmark:", err)
		c.String(500, "")
		return
	}
	c.JSON(200, gin.H{"status": 1})
}

func reorder(c *gin.Context) {
	db, err := getDB()
	if err != nil {
		log.Println("Failed to connect to database:", err)
		c.String(503, "")
		return
	}
	defer db.Close()
	session := sessions.Default(c)
	userID := session.Get("user_id")

	var reorder struct{ Old, New int }
	if err := c.BindJSON(&reorder); err != nil {
		c.String(400, "")
		return
	}

	ec := make(chan error, 1)
	var oldSeq, newSeq int
	go func() {
		ec <- db.QueryRow("SELECT seq FROM seq WHERE bookmark_id = ? AND user_id = ?",
			reorder.Old, userID).Scan(&oldSeq)
	}()
	if err := db.QueryRow("SELECT seq FROM seq WHERE bookmark_id = ? AND user_id = ?",
		reorder.New, userID).Scan(&newSeq); err != nil {
		log.Println("Failed to scan new seq:", err)
		c.String(500, "")
		return
	}
	if err := <-ec; err != nil {
		log.Println("Failed to scan old seq:", err)
		c.String(500, "")
		return
	}

	go func() {
		_, err := db.Exec("UPDATE seq SET seq = ? WHERE bookmark_id = ? AND user_id = ?",
			newSeq, reorder.Old, userID)
		ec <- err
	}()
	if oldSeq > newSeq {
		_, err = db.Exec("UPDATE seq SET seq = seq+1 WHERE seq >= ? AND seq < ? AND user_id = ?",
			newSeq, oldSeq, userID)
	} else {
		_, err = db.Exec("UPDATE seq SET seq = seq-1 WHERE seq > ? AND seq <= ? AND user_id = ?",
			oldSeq, newSeq, userID)
	}
	if err != nil {
		log.Println("Failed to update other seq:", err)
		c.String(500, "")
		return
	}
	if err := <-ec; err != nil {
		log.Println("Failed to update seq:", err)
		c.String(500, "")
		return
	}
	c.String(200, "1")
}
