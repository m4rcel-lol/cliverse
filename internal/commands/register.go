package commands

func registerAll(d *Dispatcher) {
	d.Register("post", HandlePost)
	d.Register("timeline", HandleTimeline)
	d.Register("notif", HandleNotif)
	d.Register("profile", HandleProfile)
	d.Register("follow", HandleFollow)
	d.Register("block", HandleBlock)
	d.Register("mute", HandleMute)
	d.Register("report", HandleReport)
	d.Register("settings", HandleSettings)
	d.Register("bookmark", HandleBookmark)
	d.Register("fav", HandleFav)
	d.Register("boost", HandleBoostCmd)
	d.Register("search", HandleSearch)
	d.Register("thread", HandleThread)
	d.Register("draft", HandleDraft)
	d.Register("user", HandleUser)
	d.Register("fed", HandleFed)
	d.Register("mod", HandleMod)
	d.Register("admin", HandleAdmin)
	d.Register("help", HandleHelp)
}
