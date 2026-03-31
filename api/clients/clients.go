package clients

import (
	"pingpong_server/kitex_gen/pingpong/auth/authservice"
	"pingpong_server/kitex_gen/pingpong/feed/feedservice"
	"pingpong_server/kitex_gen/pingpong/item/itemservice"
	"pingpong_server/kitex_gen/pingpong/notification/notificationservice"
	"pingpong_server/kitex_gen/pingpong/pm/pmservice"
	"pingpong_server/kitex_gen/pingpong/profile/profileservice"
)

var (
	ProfileClient      profileservice.Client
	ItemClient         itemservice.Client
	FeedClient         feedservice.Client
	AuthClient         authservice.Client
	PMClient           pmservice.Client
	NotificationClient notificationservice.Client
)
