package http

func (s *Server) createController() {
	common := s.serv.Group("/content")
	common.GET("/product/get", s.GetProducts)
	common.POST("/product/create", s.CreateProducts)
	common.POST("/product/edit", s.UpdateProduct)
	common.POST("/product/delete", s.DeleteProduct)
	common.GET("/product/get-by-id", s.GetProduct)

	common.GET("/product/modifier/list-by-filial", s.ListFilialModifiers)
	common.POST("/product/modifier/attach", s.AttachModifierToProduct)
	common.POST("/product/modifier/group/create", s.CreateModifierGroup)
	common.POST("/product/modifier/group/update", s.UpdateModifierGroup)
	common.POST("/product/modifier/group/delete", s.DeleteModifierGroup)

	eduGroup.POST("edu/filials/edu-groups", s.CreateEduGroup)
	eduGroup.POST("edu/edu-groups", s.UpdateEduGroup)
	eduGroup.POST("edu/edu-groups", s.DeleteEduGroup)
	common.GET("edu/filials/edu-groups", s.ListEduGroups)
	common.GET("edu/filials/edu-groups/get-by-id", s.GetEduGroup)

	AcademicTerm.POST("edu/filials/academic-terms", s.CreateAcademicTerm)
	AcademicTerm.POST("edu/academic-terms", s.UpdateAcademicTerm)
	AcademicTerm.POST("edu/academic-terms", s.DeleteAcademicTerm)
	common.GET("edu/filials/academic-terms", s.ListAcademicTerms)
	common.GET("edu/filials/academic-terms/get-by-id", s.GetAcademicTerm)

}
