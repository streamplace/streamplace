package atproto

// func TestChatMessage(t *testing.T) {
// 	dev := devenv.WithDevEnv(t)
// 	t.Logf("dev: %+v", dev)
// 	cli := config.CLI{
// 		PublicHost: "example.com",
// 		DBURL:      ":memory:",
// 		RelayHost:  strings.ReplaceAll(dev.PDSURL, "http://", "ws://"),
// 		PLCURL:     dev.PLCURL,
// 	}
// 	t.Logf("cli: %+v", cli)
// 	b := bus.NewBus()
// 	cli.DataDir = t.TempDir()
// 	mod, err := model.MakeDB(":memory:")
// 	require.NoError(t, err)
// 	state, err := statedb.MakeDB(&cli, nil, mod)
// 	require.NoError(t, err)
// 	atsync := &ATProtoSynchronizer{
// 		CLI:        &cli,
// 		StatefulDB: state,
// 		Model:      mod,
// 		Bus:        b,
// 	}

// 	ctx, cancel := context.WithCancel(context.Background())

// 	done := make(chan struct{})

// 	go func() {
// 		err := atsync.StartFirehose(ctx)
// 		require.NoError(t, err)
// 		close(done)
// 	}()

// 	user := dev.CreateAccount(t)
// 	user2 := dev.CreateAccount(t)

// 	ch := b.Subscribe(user.DID)
// 	defer b.Unsubscribe(user.DID, ch)

// 	busMessages := []bus.Message{}
// 	go func() {
// 		for msg := range ch {
// 			t.Logf("message: %+v", msg)
// 			busMessages = append(busMessages, msg)
// 		}
// 	}()

// 	msg := &streamplace.ChatMessage{
// 		LexiconTypeID: "place.stream.chat.message",
// 		Text:          "Hello, world!",
// 		CreatedAt:     time.Now().Add(-time.Second).Format(util.ISO8601),
// 		Streamer:      user.DID,
// 	}

// 	rec1, err := comatproto.RepoCreateRecord(ctx, user.XRPC, &comatproto.RepoCreateRecord_Input{
// 		Collection: "place.stream.chat.message",
// 		Repo:       user.DID,
// 		Record:     &lexutil.LexiconTypeDecoder{Val: msg},
// 	})
// 	require.NoError(t, err)

// 	msg2 := &streamplace.ChatMessage{
// 		LexiconTypeID: "place.stream.chat.message",
// 		Text:          "Hello, world 2!",
// 		CreatedAt:     time.Now().Format(util.ISO8601),
// 		Streamer:      user.DID,
// 	}

// 	_, err = comatproto.RepoCreateRecord(ctx, user2.XRPC, &comatproto.RepoCreateRecord_Input{
// 		Collection: "place.stream.chat.message",
// 		Repo:       user2.DID,
// 		Record:     &lexutil.LexiconTypeDecoder{Val: msg2},
// 	})
// 	require.NoError(t, err)

// 	messages := []*streamplace.ChatDefs_MessageView{}
// 	err = untilNoErrors(t, func() error {
// 		messages, err = mod.MostRecentChatMessages(user.DID)
// 		if err != nil {
// 			return err
// 		}
// 		if len(messages) != 2 {
// 			return fmt.Errorf("expected 2 messages, got %d", len(messages))
// 		}
// 		if len(busMessages) != 2 {
// 			return fmt.Errorf("expected 2 bus messages, got %d", len(busMessages))
// 		}
// 		return nil
// 	})
// 	// Reverse the messages slice to match expected order (most recent first)
// 	slices.SortFunc(messages, func(a, b *streamplace.ChatDefs_MessageView) int {
// 		aTime := a.Record.Val.(*streamplace.ChatMessage).CreatedAt
// 		bTime := b.Record.Val.(*streamplace.ChatMessage).CreatedAt
// 		if aTime < bTime {
// 			return -1
// 		} else if aTime > bTime {
// 			return 1
// 		}
// 		return 0
// 	})
// 	require.Equal(t, msg.Text, messages[0].Record.Val.(*streamplace.ChatMessage).Text)
// 	require.Equal(t, msg2.Text, messages[1].Record.Val.(*streamplace.ChatMessage).Text)
// 	busMessage1 := busMessages[0].(*streamplace.ChatDefs_MessageView)
// 	busMessage2 := busMessages[1].(*streamplace.ChatDefs_MessageView)
// 	require.Equal(t, msg.Text, busMessage1.Record.Val.(*streamplace.ChatMessage).Text)
// 	require.Equal(t, msg2.Text, busMessage2.Record.Val.(*streamplace.ChatMessage).Text)

// 	rkey := strings.TrimPrefix(rec1.Uri, fmt.Sprintf("at://%s/place.stream.chat.message/", user.DID))

// 	_, err = comatproto.RepoDeleteRecord(ctx, user.XRPC, &comatproto.RepoDeleteRecord_Input{
// 		Collection: "place.stream.chat.message",
// 		Repo:       user.DID,
// 		Rkey:       rkey,
// 	})

// 	require.NoError(t, err)

// 	err = untilNoErrors(t, func() error {
// 		messages, err = mod.MostRecentChatMessages(user.DID)
// 		if err != nil {
// 			return err
// 		}
// 		if len(messages) != 1 {
// 			return fmt.Errorf("expected 1 message, got %d", len(messages))
// 		}
// 		if len(busMessages) != 3 {
// 			return fmt.Errorf("expected 3 bus messages, got %d", len(busMessages))
// 		}
// 		return nil
// 	})
// 	require.NoError(t, err)
// 	require.Equal(t, msg2.Text, messages[0].Record.Val.(*streamplace.ChatMessage).Text)
// 	busMessage3 := busMessages[2].(*streamplace.ChatDefs_MessageView)
// 	require.Equal(t, true, *busMessage3.Deleted)

// 	cancel()
// 	<-done
// }

// func untilNoErrors(t *testing.T, f func() error) error {
// 	ticker := backoff.NewTicker(NewExponentialBackOff())
// 	defer ticker.Stop()
// 	var err error
// 	for i := 0; i < 10; i++ {
// 		err = f()
// 		if err == nil {
// 			return err
// 		}
// 		if i < 9 {
// 			<-ticker.C
// 		}
// 	}
// 	return err
// }

// // More aggressive backoff for tests
// func NewExponentialBackOff() *backoff.ExponentialBackOff {
// 	b := &backoff.ExponentialBackOff{
// 		InitialInterval:     100 * time.Millisecond,
// 		RandomizationFactor: backoff.DefaultRandomizationFactor,
// 		Multiplier:          backoff.DefaultMultiplier,
// 		MaxInterval:         2 * time.Second,
// 		MaxElapsedTime:      10 * time.Second,
// 		Clock:               backoff.SystemClock,
// 	}
// 	b.Reset()
// 	return b
// }
