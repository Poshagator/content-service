package http

func (s *Server) createController() {
	product := s.serv.Group("/content/product")
	product.GET("/get", s.productHandler.GetProducts)
	product.POST("/create", s.productHandler.CreateProducts)
	product.POST("/edit", s.productHandler.UpdateProduct)
	product.POST("/delete", s.productHandler.DeleteProduct)
	product.GET("/get-by-id", s.productHandler.GetProduct)

	product.GET("/modifier/list-by-filial", s.productHandler.ListFilialModifiers)
	product.POST("/modifier/attach", s.productHandler.AttachModifierToProduct)
	product.POST("/modifier/group/create", s.productHandler.CreateModifierGroup)
	product.POST("/modifier/group/update", s.productHandler.UpdateModifierGroup)
	product.POST("/modifier/group/delete", s.productHandler.DeleteModifierGroup)

	edu := s.serv.Group("/content/edu")
	edu.GET("/filials/edu-groups", s.eduHandler.ListEduGroups)
	edu.GET("/filials/edu-groups/get-by-id", s.eduHandler.GetEduGroup)
	edu.POST("/filials/edu-groups", s.eduHandler.CreateEduGroup)
	edu.POST("/edu-groups/update", s.eduHandler.UpdateEduGroup)
	edu.POST("/edu-groups/delete", s.eduHandler.DeleteEduGroup)

	edu.GET("/filials/academic-terms", s.eduHandler.ListAcademicTerms)
	edu.GET("/filials/academic-terms/get-by-id", s.eduHandler.GetAcademicTerm)
	edu.POST("/filials/academic-terms", s.eduHandler.CreateAcademicTerm)
	edu.POST("/academic-terms/update", s.eduHandler.UpdateAcademicTerm)
	edu.POST("/academic-terms/delete", s.eduHandler.DeleteAcademicTerm)
}
