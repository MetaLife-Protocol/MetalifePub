// SPDX-FileCopyrightText: 2021 The Go-SSB Authors
//
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"

	"go.cryptoscope.co/muxrpc/v2"
	"gopkg.in/urfave/cli.v2"

	"encoding/json"
	"go.cryptoscope.co/ssb/message"
	"go.cryptoscope.co/ssb/message/legacy"
	"go.mindeco.de/ssb-refs"
)

var streamFlags = []cli.Flag{
	&cli.IntFlag{Name: "limit", Value: -1},
	&cli.IntFlag{Name: "seq", Value: 0},
	&cli.IntFlag{Name: "gt"},
	&cli.IntFlag{Name: "lt"},
	&cli.BoolFlag{Name: "reverse"},
	&cli.BoolFlag{Name: "live"},
	&cli.BoolFlag{Name: "keys", Value: false},
	&cli.BoolFlag{Name: "values", Value: false},
	&cli.BoolFlag{Name: "private", Value: false},
}

func getStreamArgs(ctx *cli.Context) message.CreateHistArgs {
	var ref refs.FeedRef
	if id := ctx.String("id"); id != "" {
		var err error
		ref, err = refs.ParseFeedRef(id)
		if err != nil {
			panic(err)
		}
	}
	args := message.CreateHistArgs{
		ID:     ref,
		Seq:    ctx.Int64("seq"),
		AsJSON: ctx.Bool("asJSON"),
	}
	args.Limit = ctx.Int64("limit")
	args.Gt = message.RoundedInteger(ctx.Int64("gt"))
	args.Lt = message.RoundedInteger(ctx.Int64("lt"))
	args.Reverse = ctx.Bool("reverse")
	args.Live = ctx.Bool("live")
	args.Keys = ctx.Bool("keys")
	args.Values = ctx.Bool("values")
	args.Private = ctx.Bool("private")
	return args
}

var partialStreamCmd = &cli.Command{
	Name:  "partial",
	Flags: append(streamFlags, &cli.StringFlag{Name: "id"}, &cli.BoolFlag{Name: "asJSON"}),
	Action: func(ctx *cli.Context) error {
		client, err := newClient(ctx)
		if err != nil {
			return err
		}

		ir, err := client.Whoami()
		if err != nil {
			return err
		}
		id := ir.String()
		if f := ctx.String("id"); f != "" {
			id = f
		}

		src, err := client.Source(longctx, muxrpc.TypeJSON, muxrpc.Method{"partialReplication", "getMessagesOfType"}, struct {
			ID   string `json:"id"`
			Tipe string `json:"type"`
		}{
			ID:   id,
			Tipe: ctx.Args().First(),
		})
		if err != nil {
			return fmt.Errorf("source stream call failed: %w", err)
		}
		err = jsonDrain(os.Stdout, src)

		if err != nil {
			err = fmt.Errorf("byType pump failed: %w", err)
		}
		return err
	},
}

var historyStreamCmd = &cli.Command{
	Name:  "hist",
	Flags: append(streamFlags, &cli.StringFlag{Name: "id"}, &cli.BoolFlag{Name: "asJSON"}),
	Action: func(ctx *cli.Context) error {
		client, err := newClient(ctx)
		if err != nil {
			return err
		}
		ir, err := client.Whoami()
		if err != nil {
			return err
		}

		var args = getStreamArgs(ctx)
		args.ID = ir
		if f := ctx.String("id"); f != "" {
			flagRef, err := refs.ParseFeedRef(f)
			if err != nil {
				return err
			}
			args.ID = flagRef
		}
		src, err := client.Source(longctx, muxrpc.TypeJSON, muxrpc.Method{"createHistoryStream"}, args)
		if err != nil {
			return fmt.Errorf("source stream call failed: %w", err)
		}
		err = jsonDrain(os.Stdout, src)
		if err != nil {
			err = fmt.Errorf("feed hist pump failed: %w", err)
		}
		return err
	},
}

var logStreamCmd = &cli.Command{
	Name:  "log",
	Flags: streamFlags,
	Action: func(ctx *cli.Context) error {
		client, err := newClient(ctx)
		if err != nil {
			return err
		}

		var args = getStreamArgs(ctx)
		src, err := client.Source(longctx, muxrpc.TypeJSON, muxrpc.Method{"createLogStream"}, args)
		if err != nil {
			return fmt.Errorf("source stream call failed: %w", err)
		}
		err = jsonDrain1(os.Stdout, src)
		if err != nil {
			err = fmt.Errorf("message pump failed: %w", err)
		}
		return err
	},
}

var sortedStreamCmd = &cli.Command{
	Name:  "sorted",
	Flags: streamFlags,
	Action: func(ctx *cli.Context) error {
		client, err := newClient(ctx)
		if err != nil {
			return err
		}

		var args = getStreamArgs(ctx)
		src, err := client.Source(longctx, muxrpc.TypeJSON, muxrpc.Method{"createFeedStream"}, args)
		if err != nil {
			return fmt.Errorf("source stream call failed: %w", err)
		}
		err = jsonDrain(os.Stdout, src)
		if err != nil {
			err = fmt.Errorf("message pump failed: %w", err)
		}
		return err
	},
}

var typeStreamCmd = &cli.Command{
	Name:  "bytype",
	Flags: streamFlags,
	Action: func(ctx *cli.Context) error {
		client, err := newClient(ctx)
		if err != nil {
			return err
		}
		var targs message.MessagesByTypeArgs
		arg := getStreamArgs(ctx)
		targs.CommonArgs = arg.CommonArgs
		targs.StreamArgs = arg.StreamArgs
		targs.Type = ctx.Args().First()
		src, err := client.Source(longctx, muxrpc.TypeJSON, muxrpc.Method{"messagesByType"}, targs)
		if err != nil {
			return fmt.Errorf("source stream call failed: %w", err)
		}
		err = jsonDrain(os.Stdout, src)
		if err != nil {
			err = fmt.Errorf("message pump failed: %w", err)
		}
		return err
	},
}

var repliesStreamCmd = &cli.Command{
	Name:  "replies",
	Flags: append(streamFlags, &cli.StringFlag{Name: "tname", Usage: "tangle name (v2)"}),
	Action: func(ctx *cli.Context) error {
		client, err := newClient(ctx)
		if err != nil {
			return err
		}

		var targs message.TanglesArgs
		arg := getStreamArgs(ctx)
		targs.CommonArgs = arg.CommonArgs
		targs.StreamArgs = arg.StreamArgs
		targs.Root, err = refs.ParseMessageRef(ctx.Args().First())
		if err != nil {
			return err
		}
		targs.Name = ctx.String("tname")

		src, err := client.Source(longctx, muxrpc.TypeJSON, muxrpc.Method{"tangles", "replies"}, targs)
		if err != nil {
			return fmt.Errorf("source stream call failed: %w", err)
		}
		err = jsonDrain(os.Stdout, src)
		if err != nil {
			err = fmt.Errorf("message pump failed: %w", err)
		}
		return err
	},
}

var replicateUptoCmd = &cli.Command{
	Name:  "upto",
	Flags: streamFlags,
	Action: func(ctx *cli.Context) error {
		client, err := newClient(ctx)
		if err != nil {
			return err
		}
		var args = getStreamArgs(ctx)
		src, err := client.Source(longctx, muxrpc.TypeJSON, muxrpc.Method{"replicate", "upto"}, args)
		if err != nil {
			return fmt.Errorf("source stream call failed: %w", err)
		}
		err = jsonDrain(os.Stdout, src)
		if err != nil {
			err = fmt.Errorf("message pump failed: %w", err)
		}
		return err
	},
}

func jsonDrain(w io.Writer, r *muxrpc.ByteSource) error {

	var buf = &bytes.Buffer{}
	for r.Next(context.TODO()) { // read/write loop for messages

		buf.Reset()
		err := r.Reader(func(r io.Reader) error {
			_, err := buf.ReadFrom(r)
			return err
		})
		if err != nil {
			return err
		}

		// jsonReply, err := json.MarshalIndent(buf.Bytes(), "", "  ")
		// if err != nil {
		// 	return err
		// }

		_, err = buf.WriteTo(os.Stdout)
		if err != nil {
			return err
		}

	}
	return r.Err()
}

func jsonDrain1(w io.Writer, r *muxrpc.ByteSource) error {

	var buf = &bytes.Buffer{}

	TempMsgMap = make(map[string]*TempdMessage)
	Name2Hex = make(map[string]string)
	LikeCountMap = make(map[string]*LasterNumLikes)
	LikeDetail = []string{}

	for r.Next(context.TODO()) { // read/write loop for messages

		buf.Reset()
		err := r.Reader(func(r io.Reader) error {
			_, err := buf.ReadFrom(r)
			return err
		})
		if err != nil {
			return err
		}

		//jsonReply, err := json.MarshalIndent(buf.Bytes(), "", "  ")
		if err != nil {
			return err
		}

		/*_, err = buf.WriteTo(os.Stdout)
		if err != nil {
			return err
		}*/

		var msgStruct legacy.DeserializedMessage

		err = json.Unmarshal(buf.Bytes(), &msgStruct)
		if err != nil {
			fmt.Println(fmt.Sprintf("Muxrpc.ByteSource Unmarshal to json err =%s", err))
			return err
		}

		fmt.Println("******receive a message******")
		fmt.Println(fmt.Sprintf("[message]previous\t:%v", msgStruct.Previous))
		fmt.Println(fmt.Sprintf("[message]sequence\t:%v", msgStruct.Sequence))
		fmt.Println(fmt.Sprintf("[message]author\t:%v", msgStruct.Author))
		fmt.Println(fmt.Sprintf("[message]timestamp\t:%v", msgStruct.Timestamp))
		fmt.Println(fmt.Sprintf("[message]hash\t:%v", msgStruct.Hash))

		//1、记录消息ID和author的关系
		if msgStruct.Previous != nil { //这里需要过滤掉根消息Previous=null
			TempMsgMap[fmt.Sprintf("%v", msgStruct.Previous)] = &TempdMessage{
				Author: fmt.Sprintf("%v", msgStruct.Author),
			}
		}

		//2、记录like的统计结果
		contentJust := string(msgStruct.Content[0])
		if contentJust == "{" {
			//fmt.Println("??????????????????????????????????????")
			//1、like的信息
			/*
				"content":
				{
					"type":"vote",
					"vote":
					{
						"link":"%093tLb09WGxFyFfdYsgi955h/1uGxc+zCXS2b4ZCQH0=.sha256",
						"value":1,
						"expression":"Like"
					}
				},
			*/
			cvs := ContentVoteStru{}
			err = json.Unmarshal(msgStruct.Content, &cvs)
			if err == nil {
				if string(cvs.Type) == "vote" {
					fmt.Println(fmt.Sprintf("[vote]link :%v", cvs.Vote.Link))
					fmt.Println(fmt.Sprintf("[vote]expression :%v", cvs.Vote.Expression))
					//get the Like tag ,因为like肯定在发布message后,先记录被like的link，再找author
					if string(cvs.Vote.Expression) == "Like" {
						LikeDetail = append(LikeDetail, cvs.Vote.Link)
					}
				}
			} else {
				fmt.Println(fmt.Sprintf("Unmarshal for vote , err %v", err))
			}

			//3、about即修改备注名为hex-address的信息
			/*
				"content":
					{
						"type":"about",
						"about":"@/q3ohp8l7x2H5zULoCTFM8lH3TZk/ueYb8cA7LxIHyE=.ed25519",
						"name":"computer-node-patchwork"
					},
			*/
			cau := ContentAboutStru{}
			err = json.Unmarshal(msgStruct.Content, &cau)
			if err == nil {
				if string(cau.Type) == "about" {
					fmt.Println(fmt.Sprintf("[about]about :%v", cau.About))
					fmt.Println(fmt.Sprintf("[about]name :%v", cau.Name))
					Name2Hex[fmt.Sprintf("%v", cau.About)] =
						fmt.Sprintf("%v", cau.Name)

				}
			} else {
				fmt.Println(fmt.Sprintf("Unmarshal for about , err %v", err))
			}
		}

		//记录客户端id和Address的绑定关系

		fmt.Println(fmt.Sprintf("================================================================================================%v", msgStruct.Sequence))
	}
	// 编写LikeCount 被like的author收集到的点zan总数量
	//LikeCountMap=make(map[string]*LasterNumLikes)
	for _, likeLink := range LikeDetail {
		author, ok := TempMsgMap[likeLink]
		if ok {
			_, ok := LikeCountMap[author.Author]
			if !ok {
				infos := LasterNumLikes{
					ClientID:         author.Author,
					ClientAddress:    "i do not know it's Eth Address",
					LasterAddVoteNum: 0,
					LasterVoteNum:    0,
				}
				LikeCountMap[author.Author] = &infos
			} else {
				LikeCountMap[author.Author].LasterAddVoteNum++
				LikeCountMap[author.Author].LasterVoteNum++
			}

			hexStr, ok := Name2Hex[author.Author]
			if ok {
				LikeCountMap[author.Author].ClientAddress = hexStr
			}
		}

		fmt.Println("likelink:" + likeLink)

	}
	//print for test
	fmt.Println("消息ID**********发布人")
	for key := range TempMsgMap { //取map中的值
		fmt.Println(key, "**********", TempMsgMap[key].Author)
	}
	fmt.Println("客户端ID**********以太坊地址")
	for key := range Name2Hex { //取map中的值
		fmt.Println(key, "**********", Name2Hex[key])
	}
	fmt.Println("计算出的发币结果")
	fmt.Println(fmt.Sprintf("request test result:%s", LikeCountMap))

	return r.Err()
}

// LikeDetail 存储一轮搜索到的
var LikeDetail []string

// LikeCount for save message for search likes's author
var TempMsgMap map[string]*TempdMessage

// Name2Hex for save message for search likes's author
var Name2Hex map[string]string

// LikeCount for api service link(eg:%vSK7+wJ7ceZNVUCkTQliXrhgfffr5njb5swTrEZLDiM=.sha256)
var LikeCountMap map[string]*LasterNumLikes

type ContentVoteStru struct {
	Type string    `json:"type"`
	Vote *VoteStru `json:"vote"`
}
type VoteStru struct {
	Link       string `json:"link"`
	value      int    `json:"value"`
	Expression string `json:"expression"`
}

type ContentAboutStru struct {
	Type  string `json:"type"`
	About string `json:"about"`
	Name  string `json:"name"`
}

// LasterNumLikes
// ClientID 客户端ID
// ClientAddress 客户端执行的Hex address
// VoteLink为被点赞的内容ID
// LasterAddVoteNum 为新增的点赞数量
// LasterAddVoteNum 收集到了总的点赞数量（如果发放奖励在先先，有取消点赞的，不收回奖励
type LasterNumLikes struct {
	ClientID         string `json:"client_id"`
	ClientAddress    string `json:"client_eth_address"`
	LasterAddVoteNum int64  `json:"laster_add_vote_num"`
	LasterVoteNum    int64  `json:"laster_vote_num"`
	//VoteLink         []string `json:"laster_add_vote_num"`
}

// TempdMessage 用于一次搜索的结果统计
type TempdMessage struct {
	//Previous string `json:"previous"`
	Author string `json:"author"`
}
