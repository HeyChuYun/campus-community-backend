package logic

import (
	"hfjj/backend/proto/relationpb"
	"hfjj/backend/relation-service/internal/repository"
)

func toSnapshotPB(snapshot repository.RelationSnapshot) *relationpb.RelationSnapshot {
	return &relationpb.RelationSnapshot{
		ActorUserId:    snapshot.ActorUserID,
		TargetUserId:   snapshot.TargetUserID,
		IsFollowing:    snapshot.IsFollowing,
		IsFollowedBy:   snapshot.IsFollowedBy,
		IsMutualFollow: snapshot.IsMutualFollow,
		IsFriend:       snapshot.IsFriend,
		IsBlocking:     snapshot.IsBlocking,
		IsBlockedBy:    snapshot.IsBlockedBy,
	}
}

func toCounterPB(counter repository.UserCounter) *relationpb.UserCounter {
	return &relationpb.UserCounter{
		UserId:         counter.UserID,
		FollowingCount: counter.FollowingCount,
		FollowerCount:  counter.FollowerCount,
		FriendCount:    counter.FriendCount,
		BlockedCount:   counter.BlockedCount,
	}
}
