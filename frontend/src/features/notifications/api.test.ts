import { describe, expect, it } from "vitest";
import { notificationHref, notificationText, type AppNotification } from "@/features/notifications/api";

const base: AppNotification = {
  id: "n1",
  kind: "comment_reply",
  actor: { id: "u1", handle: "ann", display_name: "Ann", avatar_url: "", avatar_color: "" },
  post_id: "p1",
  post_title: "A post",
  comment_id: "c1",
  excerpt: "",
  milestone: null,
  read: false,
  created_at: "2026-10-05T00:00:00Z",
};

describe("notifications", () => {
  it("describes each kind", () => {
    expect(notificationText(base)).toBe("Ann replied to your comment");
    expect(notificationText({ ...base, kind: "mention", actor: null })).toBe("Someone mentioned you");
    expect(notificationText({ ...base, kind: "post_milestone", actor: null, milestone: 50 })).toBe(
      "Your post reached 50 upvotes",
    );
  });

  it("links to the comment when there is one", () => {
    expect(notificationHref(base)).toBe("/forums/p1#comment-c1");
    expect(notificationHref({ ...base, comment_id: null })).toBe("/forums/p1");
  });
});
