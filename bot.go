package main

import (
	"fmt"
	"math"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/rs/zerolog/pkgerrors"
)

var (
	Token string
)

func init() {
	Token = os.Getenv("TOKEN")
	if Token == "" {
		panic("empty token...")
	} else if len(Token) < 10 {
		panic("token seems too short...")
	}

	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	zerolog.TimeFieldFormat = time.RFC3339
	zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack

	consoleWriter := zerolog.ConsoleWriter{Out: os.Stdout}
	multi := zerolog.MultiLevelWriter(consoleWriter)
	log.Logger = zerolog.New(multi).With().Timestamp().Logger()

	log.Debug().Msg("Using token: " + Token[:10])
	log.Debug().
		Strs("allowed_languages", allowedLanguages).
		Int("msg_char_lim", MSG_CHAR_LIM).
		Dur("max_exectime", MAX_EXECTIME).
		Msg("Configured Settings")
}

func main() {
	// Create a new Discord session using the provided bot token.
	dg, err := discordgo.New("Bot " + Token)
	// TODO: set activity/status
	if err != nil {
		log.Error().Err(err).Msg("error creating Discord session")
		return
	}

	// Register the messageCreate func as a callback for MessageCreate events.
	dg.AddHandler(messageCreate)

	// Just like the ping pong example, we only care about receiving message
	// events in this example.
	dg.Identify.Intents = discordgo.IntentsGuildMessages

	// Open a websocket connection to Discord and begin listening.
	err = dg.Open()
	if err != nil {
		log.Error().Err(err).Msg("error opening connection")
		return
	}

	// Wait here until CTRL-C or other term signal is received.
	log.Info().Msg("Bot is now running. Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc

	// Cleanly close down the Discord session.
	dg.Close()
}

var allowedLanguages = []string{
	"awk",
	"bash",
	"befunge93",
	"brainfuck",
	"c",
	"c++",
	"cjam",
	"clojure",
	"cobol",
	"coffeescript",
	"cow",
	"crystal",
	"csharp",
	"csharp.net",
	"d",
	"dart",
	"dash",
	"dragon",
	"elixir",
	"emacs",
	"erlang",
	"file",
	"forte",
	"fortran",
	"freebasic",
	"fsharp.net",
	"fsi",
	"go",
	"golfscript",
	"groovy",
	"haskell",
	"husk",
	"iverilog",
	"japt",
	"java",
	"javascript",
	"jelly",
	"julia",
	"kotlin",
	"lisp",
	"llvm_ir",
	"lolcode",
	"lua",
	"nasm",
	"nasm64",
	"nim",
	"ocaml",
	"octave",
	"osabie",
	"paradoc",
	"pascal",
	"perl",
	"php",
	"ponylang",
	"powershell",
	"prolog",
	"pure",
	"pyth",
	"python",
	"python2",
	"racket",
	"raku",
	"retina",
	"rockstar",
	"rscript",
	"ruby",
	"rust",
	"scala",
	"sqlite3",
	"swift",
	"typescript",
	"basic",
	"basic.net",
	"vlang",
	"vyxal",
	"yeethon",
	"zig",
}

// TODO: check for exec time
const MAX_EXECTIME time.Duration = 60 * time.Second

// TODO: yeet this for SLASH commands
func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Ignore all messages created by the bot itself
	if m.Author.ID == s.State.User.ID {
		return
	}

	lines := strings.Split(m.Content, "\n")

	re := regexp.MustCompile(" +")
	split := re.Split(strings.TrimSpace(lines[0]), -1)

	cmd := split[0]
	args := split[1:]

	// TODO: add support for other commands
	if cmd != "!run" {
		return
	}

	if len(args) == 0 {
		return
	}

	log.Info().Str("user", m.Author.Username).Msg("Executed Command")

	log.Debug().Strs("lines", lines).Str("cmd", cmd).Strs("args", args).Msg("Received Command")

	// proper format: back ticks, valid language
	lang := strings.ToLower(args[0]) // spaces don't need to be trimmed since do to the way arguments are split

	validLanguage := false
	for _, l := range allowedLanguages {
		if strings.EqualFold(l, lang) {
			validLanguage = true
			break
		}
	}
	if !validLanguage {
		sendMessage(s, m.ChannelID, "Invalid input: language '"+lang+"' is not supported")
		return
	}

	var code string
	if len(lines) > 3 {
		process := lines[1:]
		if !(process[0] == "```" && process[len(process)-1] == "```") {
			sendMessage(s, m.ChannelID, "Invalid input: you must put code in code blocks")
			return
		}
		for i := 1; i < len(process)-1; i++ {
			code += process[i] + "\n"
		}
		log.Debug().Str("code", code).Msg("Running Code")
	} else if len(lines) == 3 {
		sendMessage(s, m.ChannelID, "Invalid input: do you have any code?")
		return
	} else if len(lines) < 3 {
		sendMessage(s, m.ChannelID, "Invalid input: too few lines")
		return
	}

	// exec; check max exec time
	output, err := Exec(lang, code)
	if err != nil {
		sendMessage(s, m.ChannelID, fmt.Sprintf("Encountered Error: %v", err))
		return
	}

	// crop command output is the command is too long
	template := "Received Output:\n```\n%s\n```\n"
	buffer := len(template) + 3 - 5 // adjusts for 5 extra characters which won't be counted by discord
	croppedMessage, cropped := cropMessage(output, buffer)
	if cropped > 0 {
		// length of crop message is 22 + length of number `cropped`
		template += fmt.Sprintf("(cropped %d characters)\n", cropped)
	}
	response := fmt.Sprintf(template, croppedMessage)
	log.Debug().Str("output", response).Int("cropped", cropped).Int("final_length", len(response)).Msg("Completed command output")
	sendMessage(s, m.ChannelID, response)
}

//const MSG_CHAR_LIM = 2000
const MSG_CHAR_LIM = 500 // to avoid getting rate limited

func cropMessage(toCrop string, buffer int) (string, int) {
	if len(toCrop)+buffer < MSG_CHAR_LIM {
		return toCrop, 0
	}
	// lazy crop... (could crop extra characters which might not need to be cropped)
	end := MSG_CHAR_LIM - buffer - 22
	cropped := len(toCrop) - end
	end -= int(math.Log10(float64(cropped))) + 1 // remove estimate of number of characters that the number takes
	if toCrop[end-2] == '/' && toCrop[end-3] != '/' {
		end -= 2 // remove whatever is the last
	} else if toCrop[end-1] == '/' {
		end--
	}
	return toCrop[:end], len(toCrop) - end // recalculated cropped
}

func sendMessage(session *discordgo.Session, channelID string, message string) error {
	_, err := session.ChannelMessageSend(channelID, message)
	if err != nil {
		log.Error().Err(err).Msg("Received error while sending message")
		session.ChannelMessageSend(channelID, "Experienced error sending message...")
		return err
	}
	return nil
}
