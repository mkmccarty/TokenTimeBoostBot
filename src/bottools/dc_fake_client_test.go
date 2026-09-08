package bottools

import "github.com/mkmccarty/TokenTimeBoostBot/src/dc"

// fakeDCClient is a minimal dc.Client test double. Embedding the (nil)
// interface satisfies dc.Client at compile time; only the methods a given
// test needs are overridden below. Calling any other method panics on the
// nil embedded value, which is fine because no test here exercises them.
type fakeDCClient struct {
	dc.Client

	emojisResp []dc.Emoji
	emojisErr  error

	createCalled bool
	createAppID  string
	createParams dc.EmojiParams
	createResp   *dc.Emoji
	createErr    error

	getMessageChannelID string
	getMessageID        string
	getMessageResp      *dc.MessageRef
	getMessageErr       error

	deletedChannelID string
	deletedMessageID string
	deleteErr        error
}

func (f *fakeDCClient) ApplicationEmojis(appID string) ([]dc.Emoji, error) {
	return f.emojisResp, f.emojisErr
}

func (f *fakeDCClient) ApplicationEmojiCreate(appID string, params dc.EmojiParams) (*dc.Emoji, error) {
	f.createCalled = true
	f.createAppID = appID
	f.createParams = params
	return f.createResp, f.createErr
}

func (f *fakeDCClient) GetMessage(channelID, messageID string) (*dc.MessageRef, error) {
	f.getMessageChannelID = channelID
	f.getMessageID = messageID
	return f.getMessageResp, f.getMessageErr
}

func (f *fakeDCClient) DeleteMessage(channelID, messageID string) error {
	f.deletedChannelID = channelID
	f.deletedMessageID = messageID
	return f.deleteErr
}
