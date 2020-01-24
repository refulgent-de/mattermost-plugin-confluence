package main

import (
	"fmt"
	"github.com/Brightscout/mattermost-plugin-confluence/server/config"
	"github.com/Brightscout/mattermost-plugin-confluence/server/serializer"
	"github.com/Brightscout/mattermost-plugin-confluence/server/util"
	"github.com/mattermost/mattermost-server/model"
	"github.com/mattermost/mattermost-server/plugin"
	"strings"
)

type CommandHandlerFunc func(context *model.CommandArgs, args ...string) *model.CommandResponse

type CommandHandler struct {
	handlers       map[string]CommandHandlerFunc
	defaultHandler CommandHandlerFunc
}


var confluenceCommandHandler = CommandHandler{
	handlers: map[string]CommandHandlerFunc{
		"list": listChannelSubscriptions,
		"delete": deleteSubscription,
	},
	defaultHandler: executeConflunceDefault,
}

func getCommand() *model.Command {
	return &model.Command{
		Trigger:          "confluence",
		DisplayName:      "Confluence",
		Description:      "Integration with Confluence.",
		AutoComplete:     true,
		AutoCompleteDesc: "Available commands: subscribe, list",
		AutoCompleteHint: "[command]",
	}
}

func executeConflunceDefault(context *model.CommandArgs, args ...string) *model.CommandResponse {
	return &model.CommandResponse{
		ResponseType: model.COMMAND_RESPONSE_TYPE_EPHEMERAL,
		Text:         "Invalid command",
	}
}

func postCommandResponse(context *model.CommandArgs, text string) {
	post := &model.Post{
		UserId:    config.BotUserID,
		ChannelId: context.ChannelId,
		Message:   text,
	}
	_ = config.Mattermost.SendEphemeralPost(context.UserId, post)
}

func (ch CommandHandler) Handle(context *model.CommandArgs, args ...string) *model.CommandResponse {
	for n := len(args); n > 0; n-- {
		h := ch.handlers[strings.Join(args[:n], "/")]
		if h != nil {
			return h(context, args[n:]...)
		}
	}
	return ch.defaultHandler(context, args...)
}

func (p *Plugin) ExecuteCommand(context *plugin.Context, commandArgs *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	args := strings.Fields(commandArgs.Command)
	return confluenceCommandHandler.Handle(commandArgs, args[1:]...), nil
}

func listChannelSubscriptions(context *model.CommandArgs, args ...string) *model.CommandResponse {
	channelSubscriptions := make(map[string]serializer.Subscription)
	if err := util.Get(context.ChannelId, &channelSubscriptions); err != nil {
		postCommandResponse(context, "Encountered an error getting channel subscriptions.")
		return &model.CommandResponse{}
	}
	if len(channelSubscriptions) ==0 {
		postCommandResponse(context, "No subscription found for this channel.")
		return &model.CommandResponse{}
	}
	text := fmt.Sprintf("| Alias | Base Url | Space Key | Events|\n| :----: |:--------:| :--------:| :-----:|")
	for _,subscription := range channelSubscriptions {
		text +=  fmt.Sprintf("\n|%s|%s|%s|%s|", subscription.Alias, subscription.BaseURL, subscription.SpaceKey, strings.Join(subscription.Events, ", "))
	}
	postCommandResponse(context, text)
	return &model.CommandResponse{}
}

func deleteSubscription(context *model.CommandArgs, args ...string) *model.CommandResponse {
	channelSubscriptions := make(map[string]serializer.Subscription)
	if err := util.Get(context.ChannelId, &channelSubscriptions); err != nil {
		postCommandResponse(context, fmt.Sprintf("Error occured while deleting subscription with alias **%s**.", args[0]))
		return &model.CommandResponse{}
	}
	if subscription, ok := channelSubscriptions[args[0]]; ok {
		if err := deleteSubscriptionUtil(subscription, channelSubscriptions, args[0]); err != nil {
			postCommandResponse(context, fmt.Sprintf("Error occured while deleting subscription with alias **%s**.", args[0]))
			return &model.CommandResponse{}
		}
   		postCommandResponse(context, fmt.Sprintf("Subscription with alias **%s** deleted successfully.", args[0]))
		return &model.CommandResponse{}
	} else {
		postCommandResponse(context, fmt.Sprintf("Subscription with alias **%s** not found.", args[0]))
		return &model.CommandResponse{}
	}
}

func deleteSubscriptionUtil(subscription serializer.Subscription, channelSubscriptions map[string]serializer.Subscription, alias string) error {
	key, kErr := util.GetKey(subscription.BaseURL, subscription.SpaceKey)
	if kErr != nil {
		return kErr
	}
	keySubscriptions := make(map[string][]string)
	if err := util.Get(key, &keySubscriptions); err != nil {
		return err
	}
	delete(keySubscriptions, subscription.ChannelID)
	delete(channelSubscriptions, alias)
	if err := util.Set(key, keySubscriptions); err != nil {
		return err
	}
	if err := util.Set(subscription.ChannelID, channelSubscriptions); err != nil {
		return err
	}
	return nil
}
