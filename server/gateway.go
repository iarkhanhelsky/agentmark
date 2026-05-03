package server

import "agentmark/gateway"

type (
	CommentMessage = gateway.CommentMessage
	CommentAnchor  = gateway.CommentAnchor
	CommentThread  = gateway.CommentThread
	CommentsFile   = gateway.CommentsFile
	SnapshotMeta   = gateway.SnapshotMeta
	DiffHunk       = gateway.DiffHunk
	DiffResponse   = gateway.DiffResponse
	SnapshotStore  = gateway.SnapshotStore
)

var (
	NewSnapshotStore          = gateway.NewSnapshotStore
	LoadThreads               = gateway.LoadThreads
	SaveThreads               = gateway.SaveThreads
	CommentsPathFor           = gateway.CommentsPathFor
	OpenUnresolvedThreadCount = gateway.OpenUnresolvedThreadCount
	BuildAnchor               = gateway.BuildAnchor
	ReanchorThreads           = gateway.ReanchorThreads
	ReanchorThreadsWithStore  = gateway.ReanchorThreadsWithStore
	LineDiff                  = gateway.LineDiff
	ApplyHunksToCurrent       = gateway.ApplyHunksToCurrent
	MapOldByteRangeToNew      = gateway.MapOldByteRangeToNew
)
