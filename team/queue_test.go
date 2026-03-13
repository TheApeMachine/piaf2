package team

import (
	"testing"

	"github.com/smartystreets/goconvey/convey"
)

func TestQueuePublishSnapshot(t *testing.T) {
	convey.Convey("Given a Queue", t, func() {
		queue := NewQueue(16)

		convey.Convey("Snapshot is initially empty", func() {
			convey.So(len(queue.Snapshot()), convey.ShouldEqual, 0)
		})

		convey.Convey("Publish adds to snapshot", func() {
			queue.Publish(QueueMessage{Kind: FileLock, Path: "foo.go", Agent: "Dev1"})
			snap := queue.Snapshot()
			convey.So(len(snap), convey.ShouldEqual, 1)
			convey.So(snap[0].Kind, convey.ShouldEqual, FileLock)
			convey.So(snap[0].Path, convey.ShouldEqual, "foo.go")
		})
	})
}

func TestQueueHasFileLock(t *testing.T) {
	convey.Convey("Given a Queue with file locks", t, func() {
		queue := NewQueue(16)
		queue.Publish(QueueMessage{Kind: FileLock, Path: "pkg/foo.go", Agent: "Dev1"})

		convey.Convey("HasFileLock returns true for exact path", func() {
			convey.So(queue.HasFileLock("pkg/foo.go", "Dev2"), convey.ShouldBeTrue)
		})

		convey.Convey("HasFileLock returns false when excluding the locking agent", func() {
			convey.So(queue.HasFileLock("pkg/foo.go", "Dev1"), convey.ShouldBeFalse)
		})

		convey.Convey("HasFileLock returns false for unrelated path", func() {
			convey.So(queue.HasFileLock("other.go", "Dev2"), convey.ShouldBeFalse)
		})
	})
}

func BenchmarkQueuePublish(b *testing.B) {
	queue := NewQueue(4096)
	msg := QueueMessage{Kind: FileLock, Path: "file.go", Agent: "Dev1"}

	for index := 0; index < b.N; index++ {
		queue.Publish(msg)
	}
}
