package http

// createController registers all public endpoints in a single place,
// following the same style as the product-service router.
func (s *Server) createController() {
	product := s.serv.Group("/content/product")
	product.GET("/get", s.GetProducts)
	product.POST("/create", s.CreateProducts)
	product.POST("/edit", s.UpdateProduct)
	product.POST("/delete", s.DeleteProduct)
	product.GET("/get-by-id", s.GetProduct)

	product.GET("/modifier/list-by-filial", s.ListFilialModifiers)
	product.POST("/modifier/attach", s.AttachModifierToProduct)
	product.POST("/modifier/group/create", s.CreateModifierGroup)
	product.POST("/modifier/group/update", s.UpdateModifierGroup)
	product.POST("/modifier/group/delete", s.DeleteModifierGroup)

	edu := s.serv.Group("/content/edu")
	edu.GET("/filials/edu-groups", s.ListEduGroups)
	edu.GET("/filials/edu-groups/get-by-id", s.GetEduGroup)
	edu.POST("/filials/edu-groups", s.CreateEduGroup)
	edu.POST("/edu-groups/update", s.UpdateEduGroup)
	edu.POST("/edu-groups/delete", s.DeleteEduGroup)

	edu.GET("/filials/academic-terms", s.ListAcademicTerms)
	edu.GET("/filials/academic-terms/get-by-id", s.GetAcademicTerm)
	edu.POST("/filials/academic-terms", s.CreateAcademicTerm)
	edu.POST("/academic-terms/update", s.UpdateAcademicTerm)
	edu.POST("/academic-terms/delete", s.DeleteAcademicTerm)
}
