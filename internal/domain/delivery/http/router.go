package http

func (s *Server) createController() {
	product := s.serv.Group("/content/product")
	product.GET("/list", s.productHandler.GetProducts)
	product.GET("/get", s.productHandler.GetProduct)
	product.POST("/create", s.productHandler.CreateProducts)
	product.POST("/update", s.productHandler.UpdateProduct)
	product.POST("/delete", s.productHandler.DeleteProduct)

	product.GET("/modifier/list", s.productHandler.ListFilialModifiers)
	product.POST("/modifier/attach", s.productHandler.AttachModifierToProduct)
	product.POST("/modifier/group/create", s.productHandler.CreateModifierGroup)
	product.POST("/modifier/group/update", s.productHandler.UpdateModifierGroup)
	product.POST("/modifier/group/delete", s.productHandler.DeleteModifierGroup)

	edu := s.serv.Group("/content/edu")

	// Edu Groups
	edu.GET("/group/list", s.eduHandler.ListEduGroups)
	edu.GET("/group/search", s.eduHandler.SearchEduGroups)
	edu.GET("/group/get", s.eduHandler.GetEduGroup)
	edu.POST("/group/create", s.eduHandler.CreateEduGroup)
	edu.POST("/group/update", s.eduHandler.UpdateEduGroup)
	edu.POST("/group/delete", s.eduHandler.DeleteEduGroup)

	// Academic Terms
	edu.GET("/term/list", s.eduHandler.ListAcademicTerms)
	edu.GET("/term/get", s.eduHandler.GetAcademicTerm)
	edu.POST("/term/create", s.eduHandler.CreateAcademicTerm)
	edu.POST("/term/update", s.eduHandler.UpdateAcademicTerm)
	edu.POST("/term/delete", s.eduHandler.DeleteAcademicTerm)

	// Timetable
	edu.GET("/timetable/list", s.eduHandler.GetTimetable)
	edu.POST("/sync-to-planner", s.eduHandler.SyncToPlanner)
	edu.POST("/unsubscribe-from-planner", s.eduHandler.UnsubscribeFromPlanner)
}
