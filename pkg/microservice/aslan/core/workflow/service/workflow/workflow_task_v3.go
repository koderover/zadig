package workflow

//func CreateWorkflowTaskV3(args *commonmodels.WorkflowV3Args, username, reqID string, log *zap.SugaredLogger) (*CreateTaskResp, error) {
//	pipeline, err := commonrepo.NewPipelineColl().Find(&commonrepo.PipelineFindOption{Name: args.PipelineName})
//	if err != nil {
//		log.Errorf("PipelineV2.Find %s error: %v", args.PipelineName, err)
//		return nil, e.ErrCreateTask.AddDesc(e.FindPipelineErrMsg)
//	}
//
//	nextTaskID, err := commonrepo.NewCounterColl().GetNextSeq(fmt.Sprintf(setting.PipelineTaskFmt, args.Name))
//	if err != nil {
//		log.Errorf("Counter.GetNextSeq error: %v", err)
//		return nil, e.ErrGetCounter.AddDesc(err.Error())
//	}
//
//	// 更新单服务工作的subtask的build_os
//	// 自定义基础镜像的镜像名称可能会被更新，需要使用ID获取最新的镜像名称
//	for i, subTask := range pipeline.SubTasks {
//		pre, err := base.ToPreview(subTask)
//		if err != nil {
//			log.Errorf("subTask.ToPreview error: %v", err)
//			continue
//		}
//		switch pre.TaskType {
//		case config.TaskBuildV3:
//			build, err := base.ToBuildTask(subTask)
//			if err != nil || build == nil {
//				log.Errorf("subTask.ToBuildTask error: %v", err)
//				continue
//			}
//			if build.ImageID == "" {
//				continue
//			}
//			basicImage, err := commonrepo.NewBasicImageColl().Find(build.ImageID)
//			if err != nil {
//				log.Errorf("BasicImage.Find failed, id:%s, err:%v", build.ImageID, err)
//				continue
//			}
//			build.BuildOS = basicImage.Value
//
//			// 创建任务时可以临时编辑环境变量，需要将pipeline中的环境变量更新成新的值。
//			if args.BuildArgs != nil {
//				build.JobCtx.EnvVars = args.BuildArgs
//			}
//
//			pipeline.SubTasks[i], err = build.ToSubTask()
//			if err != nil {
//				log.Errorf("build.ToSubTask error: %v", err)
//				continue
//			}
//		}
//	}
//
//	var defaultStorageURI string
//	if defaultS3, err := s3.FindDefaultS3(); err == nil {
//		defaultStorageURI, err = defaultS3.GetEncryptedURL()
//		if err != nil {
//			return nil, e.ErrS3Storage.AddErr(err)
//		}
//	}
//
//	pt := &task.Task{
//		TaskID:        nextTaskID,
//		ProductName:   pipeline.ProductName,
//		PipelineName:  args.Name,
//		Type:          config.WorkflowTypeV3,
//		TaskCreator:   username,
//		ReqID:         reqID,
//		Status:        config.StatusCreated,
//		SubTasks:      pipeline.SubTasks,
//		TeamName:      pipeline.TeamName,
//		ConfigPayload: commonservice.GetConfigPayload(0),
//		MultiRun:      pipeline.MultiRun,
//		StorageURI:    defaultStorageURI,
//	}
//
//	sort.Sort(ByTaskKind(pt.SubTasks))
//
//	for i, t := range pt.SubTasks {
//		preview, err := base.ToPreview(t)
//		if err != nil {
//			continue
//		}
//		if preview.TaskType != config.TaskDeploy {
//			continue
//		}
//
//		t, err := base.ToDeployTask(t)
//		if err == nil && t.Enabled {
//			env, err := commonrepo.NewProductColl().FindEnv(&commonrepo.ProductEnvFindOptions{
//				Namespace: pt.TaskArgs.Deploy.Namespace,
//				Name:      pt.ProductName,
//			})
//
//			if err != nil {
//				return nil, e.ErrCreateTask.AddDesc(
//					e.EnvNotFoundErrMsg + ": " + pt.TaskArgs.Deploy.Namespace,
//				)
//			}
//
//			t.EnvName = env.EnvName
//			t.ProductName = pt.ProductName
//			pt.ConfigPayload.DeployClusterID = env.ClusterID
//			pt.SubTasks[i], _ = t.ToSubTask()
//		}
//	}
//
//	repos, err := commonrepo.NewRegistryNamespaceColl().FindAll(&commonrepo.FindRegOps{})
//	if err != nil {
//		return nil, e.ErrCreateTask.AddErr(err)
//	}
//
//	pt.ConfigPayload.RepoConfigs = make(map[string]*commonmodels.RegistryNamespace)
//	for _, repo := range repos {
//		pt.ConfigPayload.RepoConfigs[repo.ID.Hex()] = repo
//	}
//
//	if err := ensurePipelineTask(pt, pt.TaskArgs.Deploy.Namespace, log); err != nil {
//		log.Errorf("Service.ensurePipelineTask failed %v %v", args, err)
//		if err, ok := err.(*ContainerNotFound); ok {
//			return nil, e.NewWithExtras(
//				e.ErrCreateTaskFailed,
//				"container doesn't exists", map[string]interface{}{
//					"productName":   err.ProductName,
//					"envName":       err.EnvName,
//					"serviceName":   err.ServiceName,
//					"containerName": err.Container,
//				})
//		}
//
//		return nil, e.ErrCreateTask.AddDesc(err.Error())
//	}
//
//	if len(pt.SubTasks) <= 0 {
//		return nil, e.ErrCreateTask.AddDesc(e.PipelineSubTaskNotFoundErrMsg)
//	}
//
//	if config.EnableGitCheck() {
//		if err := createGitCheck(pt, log); err != nil {
//			log.Error(err)
//		}
//	}
//
//	// send to queue to execute task
//	if err := CreateTask(pt); err != nil {
//		log.Error(err)
//		return nil, e.ErrCreateTask
//	}
//
//	resp := &CreateTaskResp{
//		PipelineName: args.Name,
//		TaskID:       nextTaskID,
//	}
//	return resp, nil
//}
