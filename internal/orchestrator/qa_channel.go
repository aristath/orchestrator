package orchestrator

import (
	"context"
)

// Question represents a question from a satellite agent to the orchestrator.
type Question struct {
	TaskID     string
	Content    string
	responseCh chan Answer
}

// Answer represents the orchestrator's response to a question.
type Answer struct {
	Content string
	Error   error
}

// AnswerFunc is a callback that the orchestrator provides for answering questions
// using its full plan context.
type AnswerFunc func(ctx context.Context, taskID string, question string) (string, error)

// QAChannel manages non-blocking question-and-answer communication between
// satellite agents and the orchestrator.
type QAChannel struct {
	questionCh chan Question
	answerFn   AnswerFunc
	done       chan struct{}
}

// NewQAChannel creates a new Q&A channel with the specified buffer size and answer function.
// bufferSize should typically be 2x the concurrency limit to prevent blocking.
func NewQAChannel(bufferSize int, answerFn AnswerFunc) *QAChannel {
	return &QAChannel{
		questionCh: make(chan Question, bufferSize),
		answerFn:   answerFn,
		done:       make(chan struct{}),
	}
}

// Start launches the question handler goroutine.
// It processes questions until the context is cancelled.
func (qac *QAChannel) Start(ctx context.Context) {
	go qac.handleQuestions(ctx)
}

// handleQuestions processes incoming questions until context is cancelled.
func (qac *QAChannel) handleQuestions(ctx context.Context) {
	defer close(qac.done)

	for {
		select {
		case <-ctx.Done():
			return
		case q := <-qac.questionCh:
			// Process the question
			content, err := qac.answerFn(ctx, q.TaskID, q.Content)

			// Check if context was cancelled during answer
			select {
			case <-ctx.Done():
				// Send cancellation error instead
				q.responseCh <- Answer{
					Content: "",
					Error:   ctx.Err(),
				}
				return
			default:
				// Send the answer
				q.responseCh <- Answer{
					Content: content,
					Error:   err,
				}
			}
		}
	}
}

// Ask sends a question to the orchestrator and waits for an answer.
// It respects context cancellation at both the send and receive stages.
func (qac *QAChannel) Ask(ctx context.Context, taskID string, question string) (string, error) {
	// Create buffered response channel to prevent handler blocking
	responseCh := make(chan Answer, 1)

	q := Question{
		TaskID:     taskID,
		Content:    question,
		responseCh: responseCh,
	}

	// Send question (or cancel)
	select {
	case qac.questionCh <- q:
		// Question sent successfully
	case <-ctx.Done():
		return "", ctx.Err()
	}

	// Wait for answer (or cancel)
	select {
	case answer := <-responseCh:
		if answer.Error != nil {
			return "", answer.Error
		}
		return answer.Content, nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

// Stop blocks until the handler goroutine has exited.
func (qac *QAChannel) Stop() {
	<-qac.done
}
