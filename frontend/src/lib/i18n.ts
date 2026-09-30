// Codritium i18n — English-first with zh placeholder for future locale switch.
//
// Usage:
//   import { t } from "@/lib/i18n";
//   <button>{t("submit")}</button>
//   toast.error(t("gemini_unavailable"));
//
// Rules (see /Users/johns3248/project/AICH/Codritium/.claude/CLAUDE.md):
//   - All user-facing strings MUST go through t(). No bare literals like
//     <button>Submit</button>.
//   - Default locale is "en". The zh map is reserved for future locale-switch
//     UI; do NOT add user-facing zh strings during MVP unless explicitly part
//     of the i18n-switcher feature.

type LocaleKey =
  // generic
  | "loading"
  | "submit"
  | "cancel"
  | "close"
  | "restored_from_session"
  // navigation
  | "problems_title"
  | "problems_subtitle"
  | "problems_loading"
  // editor
  | "open_a_file_first"
  // chat
  | "chat_send_failed"
  | "chat_input_disabled_no_grader"
  | "ask_ai"
  | "ai_thinking"
  | "ai_role"
  | "chat_subtitle"
  // tips-agent tab
  | "ask_tutor"
  | "tutor_thinking"
  | "tutor_role"
  | "tutor_empty_state"
  | "agent_tab_label"
  | "tutor_tab_label"
  | "tutor_submitted"
  | "tutor_nudge_agent"
  | "tutor_send_failed"
  // settings
  | "settings_link"
  | "settings_title"
  | "settings_subtitle"
  | "settings_layout_section"
  | "settings_layout_hint"
  | "settings_font_section"
  | "settings_font_hint"
  | "settings_tips_visibility_section"
  | "settings_tips_visibility_hint"
  | "settings_coming_soon"
  // grader / submit
  | "submit_failed"
  | "grading_failed"
  | "gemini_unavailable"
  | "gemini_unavailable_progress_saved"
  | "submit_disabled_no_grader"
  // auth
  | "login_title"
  | "password_placeholder"
  | "password_required"
  | "sign_in"
  | "sign_out"
  | "signing_in"
  | "login_failed"
  | "register_card_title"
  | "register_card_desc"
  | "register_password_hint"
  | "create_account"
  | "register_prompt"
  | "login_prompt"
  | "auth_email_exists"
  | "auth_invalid_credentials"
  | "auth_invalid_input"
  | "auth_request_failed"
  // session
  | "session_quota_exceeded_oldest_evicted"
  // apply
  | "apply_failed"
  | "apply_in_progress"
  // submissions list
  | "submissions_deleted"
  | "submissions_delete_partial"
  // reply / answer mode (post-submission replay)
  | "reply_page_title"
  | "reply_subtitle"
  | "reply_back_link"
  | "reply_mode_tab"
  | "answer_mode_tab"
  | "reply_track_official"
  | "reply_track_mine"
  | "reply_empty_official_full"
  | "reply_no_official_short"
  | "reply_empty_mine"
  | "reply_empty_official"
  | "diff_select_hint"
  | "diff_no_content"
  | "diff_select_from_left"
  | "solution_explanation"
  | "explanation_empty"
  | "filetree_files_header"
  | "filetree_modified_label"
  | "step_empty"
  | "nav_prev"
  | "nav_next"
  | "envelope_session_started"
  | "envelope_session_submitted"
  | "envelope_plan_in"
  | "envelope_plan_out"
  | "envelope_propose_tool"
  | "envelope_propose_with_tool"
  | "envelope_tool_result"
  | "envelope_reasoning"
  | "envelope_approved"
  | "envelope_rejected"
  | "envelope_pushed_back"
  | "envelope_tests_executed"
  | "envelope_self_check"
  | "envelope_reverted"
  | "role_engineer"
  // home / landing
  | "dim_correctness_title"
  | "dim_correctness_desc"
  | "dim_decomposition_title"
  | "dim_decomposition_desc"
  | "dim_ai_collab_title"
  | "dim_ai_collab_desc"
  | "dim_verification_title"
  | "dim_verification_desc"
  // app bar
  | "brand_name"
  | "appbar_readme"
  | "appbar_help"
  // chat panel
  | "chat_ai_assistant"
  | "chat_role_you"
  | "chat_apply_btn"
  | "chat_enter_to_queue_hint"
  | "chat_enter_to_send_hint"
  | "chat_queue_btn"
  | "chat_send_btn"
  | "chat_tab_enter"
  | "chat_queued_prefix"
  | "chat_proposed_word"
  | "patch_label_auto"
  | "patch_label_approved"
  | "patch_label_modified"
  | "patch_label_rejected"
  // patch preview
  | "patch_decision_failed"
  | "patch_approve"
  | "patch_modify"
  | "patch_reject"
  | "patch_reject_confirm"
  | "patch_reject_reason_placeholder"
  | "patch_save_approve"
  | "patch_runcommand_warn"
  // pending patches sidebar
  | "pending_label"
  // login (additions to existing login keys)
  | "login_card_title"
  | "login_card_desc"
  | "login_account_label"
  | "login_password_placeholder_hint"
  | "login_footer_info"
  // problem-workspace tabs
  | "problem_tab_brief"
  | "problem_tab_hint"
  | "problem_tab_anti_patterns"
  | "problem_tab_discussion"
  | "problem_tab_solutions"
  | "antipattern_hands_off_name"
  | "antipattern_hands_off_desc"
  | "antipattern_feature_marathon_name"
  | "antipattern_feature_marathon_desc"
  | "antipattern_ai_showcase_name"
  | "antipattern_ai_showcase_desc"
  | "antipattern_not_thinking_name"
  | "antipattern_not_thinking_desc"
  // chat-placeholder + chat-drawer-toggle
  | "agent_placeholder_system"
  | "agent_placeholder_user"
  | "agent_placeholder_agent"
  // u/[handle] public profile
  | "streak_current_label"
  | "streak_active_today"
  | "streak_last_active_fmt"
  | "no_activity_recorded"
  | "streak_all_time_best"
  | "tier_pro"
  | "tier_free"
  | "pro_tier_desc"
  | "public_history_title"
  | "public_history_desc"
  | "browse_recent_forum_posts_prefix"
  | "this_user_via_forum"
  | "filter_ui_lands_v06"
  // profile
  | "profile_category_all"
  | "profile_category_debugging"
  | "profile_category_refactoring"
  | "profile_category_feature_build"
  | "profile_category_security"
  | "profile_category_company_premium"
  // forum
  | "forum_section_interview_label"
  | "forum_section_career_label"
  | "forum_section_compensation_label"
  | "forum_section_feedback_label"
  | "forum_section_problems_label"
  | "forum_for_you"
  | "forum_sections"
  | "forum_create"
  | "forum_sort_votes"
  | "forum_sort_newest"
  | "forum_sort_best"
  | "forum_sort_comments"
  | "forum_search_placeholder"
  | "forum_results_for_fmt"
  | "forum_clear_filter"
  | "forum_trending"
  | "forum_trending_empty"
  | "forum_pinned"
  | "forum_empty_feed"
  | "forum_load_more"
  | "forum_load_more_comments"
  | "forum_anonymous"
  | "forum_verified"
  | "forum_posted_anonymously"
  | "forum_votes"
  | "forum_views"
  | "forum_comments"
  | "forum_comments_count_fmt"
  | "forum_upvote"
  | "forum_downvote"
  | "forum_post_actions"
  | "forum_copy_link"
  | "forum_link_copied"
  | "forum_edit"
  | "forum_delete"
  | "forum_pin"
  | "forum_unpin"
  | "forum_post_pinned"
  | "forum_post_unpinned"
  | "forum_delete_post_confirm"
  | "forum_post_deleted"
  | "forum_edited"
  | "forum_reply"
  | "forum_op_badge"
  | "forum_no_comments"
  | "forum_comment_placeholder"
  | "forum_reply_placeholder"
  | "forum_post_comment"
  | "forum_comment_anonymously"
  | "forum_comment_deleted"
  | "forum_delete_comment_confirm"
  | "forum_login_link"
  | "forum_login_to_comment"
  | "forum_new_post_title"
  | "forum_edit_post_title"
  | "forum_title_label"
  | "forum_title_placeholder"
  | "forum_title_hint_fmt"
  | "forum_section_label"
  | "forum_problem_label"
  | "forum_problem_none"
  | "forum_tags_label_fmt"
  | "forum_tags_placeholder"
  | "forum_remove_tag_fmt"
  | "forum_write"
  | "forum_preview"
  | "forum_markdown_hint"
  | "forum_body_label"
  | "forum_body_placeholder"
  | "forum_preview_empty"
  | "forum_post_anonymously"
  | "forum_publish"
  | "forum_save"
  | "forum_post_published"
  | "forum_post_updated"
  | "forum_err_login_required"
  | "forum_err_invalid_title"
  | "forum_err_invalid_body"
  | "forum_err_invalid_tag"
  | "forum_err_too_many_tags"
  | "forum_err_unknown_problem"
  | "forum_err_anonymous_not_allowed"
  | "forum_err_not_found"
  | "forum_err_generic"
  | "forum_just_now"
  | "forum_minutes_ago_fmt"
  | "forum_hours_ago_fmt"
  | "forum_days_ago_fmt"
  // SubmissionsList
  | "submissions_empty_prefix"
  | "submissions_problem_list_word"
  | "submissions_total_fmt"
  | "submissions_delete_btn"
  | "submissions_delete_btn_count_fmt"
  | "submissions_select_btn"
  | "submissions_unknown_problem"
  | "submissions_select_row_aria_fmt"
  // nav
  | "nav_problems"
  | "nav_forum"
  | "nav_forums"
  | "nav_profile"
  | "nav_notifications"
  | "menu_plans"
  // dashboard
  // profile identity
  // forums placeholder
  // ResumeOrFresh
  | "resume_just_now"
  | "resume_minute_ago"
  | "resume_minutes_ago_fmt"
  | "resume_hour_ago"
  | "resume_hours_ago_fmt"
  | "resume_day_ago"
  | "resume_days_ago_fmt"
  | "resume_start_solving_btn"
  | "resume_start_fresh_btn"
  | "resume_continue_btn"
  | "resume_last_edited_fmt"
  // anti-pattern-chip
  | "antipattern_label_hands_off"
  | "antipattern_label_feature_marathon"
  | "antipattern_label_ai_showcase"
  | "antipattern_label_not_thinking"
  | "antipattern_flagged"
  | "antipattern_clear"
  | "antipattern_not_evaluated"
  // problems list
  | "problems_count_seed_fmt"
  | "problems_count_fmt"
  | "problems_filter_category"
  | "problems_filter_difficulty"
  | "problems_filter_company"
  | "problems_filter_all"
  | "problems_difficulty_easy"
  | "problems_difficulty_medium"
  | "problems_difficulty_hard"
  | "problems_no_match"
  | "problems_clear_filters"
  | "problems_pro_lock_tooltip"
  | "problems_filter_status"
  | "problems_status_todo"
  | "problems_status_attempted"
  | "problems_status_solved"
  | "problems_col_title"
  | "problems_search_placeholder"
  | "problems_solved_count_fmt"
  | "problems_page_indicator_fmt"
  | "problems_page_prev"
  | "problems_page_next"
  | "problems_page_word"
  | "problems_page_jump_aria"
  | "submission_total_out_of_100"
  | "pro_badge"
  // problem brief
  | "breadcrumb_problems"
  | "back_to_problems"
  | "problem_intro_blurb"
  | "problem_tab_description"
  | "problem_tab_submissions"
  | "problem_solutions_unlocked_desc"
  | "problem_solutions_view_btn"
  | "problem_submissions_empty"
  | "problem_submission_status_pending"
  | "problem_submission_status_grading"
  | "problem_submission_status_failed"
  | "problem_start_title"
  | "problem_start_point_files"
  | "problem_start_point_ai"
  | "problem_start_point_timer"
  | "problem_solutions_desc"
  // submission detail
  | "dim_label_correctness"
  | "dim_label_decomposition"
  | "dim_label_ai_collab"
  | "dim_label_verification"
  | "dim_label_communication"
  | "submission_replay_btn"
  | "submission_iterations_title"
  | "submission_resubmit_btn"
  | "submission_not_graded_title"
  | "submission_grading_body"
  | "submission_failed_body"
  | "submission_judge_prefix"
  | "submission_meta_submitted_fmt"
  | "submission_five_dim_title"
  | "submission_five_dim_desc_prefix"
  | "submission_code_grader_title"
  | "submission_code_grader_desc_fmt"
  | "submission_anti_pattern_title"
  | "submission_anti_pattern_clean"
  | "submission_anti_pattern_count_fmt"
  | "submission_anti_pattern_unevaluated"
  | "submission_metadata_title"
  | "submission_meta_ai_tool"
  | "submission_meta_model"
  | "submission_meta_prompts"
  | "submission_meta_id"
  | "submission_meta_lines_changed"
  // landing
  // feature cards (six)
  // profile + tier + credits + badge
  // comments
  | "comments_title"
  | "comments_empty"
  | "comments_post_placeholder"
  | "comments_post_btn"
  | "comments_load_more"
  | "comments_reply_btn"
  | "comments_delete_btn"
  | "comments_must_complete"
  // notes
  | "notes_title"
  | "notes_empty"
  | "notes_create_placeholder"
  | "notes_save_btn"
  | "notes_share_btn"
  | "notes_share_success"
  | "notes_share_failed"
  // reply / report tabs
  | "reply_tab_label"
  | "report_tab_label"
  | "official_solution_title"
  | "official_solution_subtitle"
  | "official_locked_title"
  | "official_locked_body"
  | "official_back_link"
  | "view_official_solution"
  | "reply_mode_official"
  | "reply_mode_user"
  | "reply_loading"
  // tips highlight
  | "tips_highlight_label"
  | "tips_first_visit_hint"
  // README float + IDE multi-pane
  | "readme_float_btn"
  | "readme_dock_btn"
  | "readme_floating_title"
  | "readme_floating_hint"
  | "logout_confirm_title"
  | "logout_confirm_body"
  | "logout_confirm_action"
  | "workspace_exit_title"
  | "workspace_exit_body"
  | "workspace_exit_action"
  | "workspace_exit_aria"
  | "ide_split_btn"
  | "ide_close_pane_btn"
  | "ide_pane_limit_reached"
  // home page
  | "home_hero_eyebrow"
  | "home_hero_title"
  | "home_hero_blurb"
  | "home_cta_create_account"
  | "home_cta_continue"
  | "home_cta_browse"
  | "home_stat_problems"
  | "home_stat_tracks"
  | "home_stat_dimensions"
  | "home_mock_user_msg"
  | "home_mock_ai_msg"
  | "home_mock_patch"
  | "home_mock_approve"
  | "home_mock_reject"
  | "home_mock_report"
  | "home_explore_title"
  | "home_explore_blurb"
  | "home_feature_problems_title"
  | "home_feature_problems_desc"
  | "home_feature_problems_link"
  | "home_feature_agent_title"
  | "home_feature_agent_desc"
  | "home_feature_agent_link"
  | "home_feature_scoring_title"
  | "home_feature_scoring_desc"
  | "home_feature_scoring_link"
  | "home_visual_problem_count"
  | "home_visual_patch_title"
  | "home_visual_patch_hint"
  | "home_visual_patch_edit"
  | "home_visual_antipatterns_clear"
  | "home_candidates_title"
  | "home_candidates_desc"
  | "home_candidates_link_problems"
  | "home_candidates_link_forums"
  | "home_teams_title"
  | "home_teams_desc"
  | "home_teams_point_rubric"
  | "home_teams_point_replay"
  | "home_teams_point_problems"
  | "home_teams_link"
  | "home_showcase_title"
  | "home_showcase_blurb"
  | "home_showcase_copy"
  | "home_showcase_copied"
  | "home_showcase_open"
  | "home_showcase_browse_all"
  | "home_mission_title"
  | "home_mission_body"
  | "home_mission_skills_label"
  | "home_skill_incidents"
  | "home_skill_security"
  | "home_skill_refactor"
  | "home_skill_patches"
  | "home_skill_tests"
  | "home_skill_tradeoffs"
  | "home_hiring_title"
  | "home_hiring_body"
  | "home_hiring_link"
  | "home_footer_copyright"
  | "home_footer_contact"
  // profile page
  | "prof_member_since"
  | "prof_edit_btn"
  | "prof_edit_title"
  | "prof_edit_name"
  | "prof_edit_name_required"
  | "prof_edit_bio"
  | "prof_edit_bio_placeholder"
  | "prof_edit_region"
  | "prof_edit_region_placeholder"
  | "prof_edit_save"
  | "prof_edit_saving"
  | "prof_edit_saved"
  | "prof_edit_failed"
  | "prof_community_title"
  | "prof_stat_posts"
  | "prof_stat_comments"
  | "prof_stat_upvotes"
  | "prof_last_week"
  | "prof_tracks_title"
  | "prof_strengths_title"
  | "prof_strength_strong"
  | "prof_strength_developing"
  | "prof_strength_focus"
  | "prof_strength_samples"
  | "prof_not_enough_data"
  | "prof_solved_label"
  | "prof_attempting"
  | "prof_badges_title"
  | "prof_badge_next"
  | "prof_badge_locked"
  | "prof_badge_early"
  | "prof_badge_first"
  | "prof_badge_ten"
  | "prof_badge_streak"
  | "prof_badge_tracks"
  | "prof_badge_ninety"
  | "prof_badge_fifty"
  | "prof_activity_title"
  | "prof_active_days"
  | "prof_current_streak"
  | "prof_max_streak"
  | "prof_day_tooltip"
  | "prof_tabs_label"
  | "prof_tab_recent"
  | "prof_tab_solved"
  | "prof_tab_posts"
  | "prof_tab_all"
  | "prof_view_all"
  | "prof_status_grading"
  | "prof_status_failed"
  | "prof_best_score"
  | "prof_empty_submissions"
  | "prof_empty_solved"
  | "prof_empty_posts"
  | "prof_empty_cta"
  | "prof_empty_posts_cta";

type Locale = "en" | "zh";

const STRINGS: Record<Locale, Record<LocaleKey, string>> = {
  en: {
    prof_member_since: "Joined {date}",
    prof_edit_btn: "Edit profile",
    prof_edit_title: "Edit profile",
    prof_edit_name: "Display name",
    prof_edit_name_required: "Display name can't be empty.",
    prof_edit_bio: "Bio",
    prof_edit_bio_placeholder: "What are you practicing for?",
    prof_edit_region: "Location",
    prof_edit_region_placeholder: "City or country",
    prof_edit_save: "Save",
    prof_edit_saving: "Saving…",
    prof_edit_saved: "Profile updated",
    prof_edit_failed: "Couldn't save your profile. Please try again.",
    prof_community_title: "Community stats",
    prof_stat_posts: "Posts",
    prof_stat_comments: "Comments",
    prof_stat_upvotes: "Upvotes",
    prof_last_week: "Last week",
    prof_tracks_title: "Tracks",
    prof_strengths_title: "Strengths",
    prof_strength_strong: "Strong",
    prof_strength_developing: "Developing",
    prof_strength_focus: "Needs work",
    prof_strength_samples: "Average of {n} graded submissions",
    prof_not_enough_data: "Not enough data",
    prof_solved_label: "Solved",
    prof_attempting: "{n} Attempting",
    prof_badges_title: "Badges",
    prof_badge_next: "Next badge",
    prof_badge_locked: "(locked)",
    prof_badge_early: "Early Access",
    prof_badge_first: "First Submission",
    prof_badge_ten: "10 Problems Solved",
    prof_badge_streak: "7-Day Streak",
    prof_badge_tracks: "Every Track",
    prof_badge_ninety: "Score 90+",
    prof_badge_fifty: "50 Problems Solved",
    prof_activity_title: "submissions in the past year",
    prof_active_days: "Active days:",
    prof_current_streak: "Current streak:",
    prof_max_streak: "Max streak:",
    prof_day_tooltip: "{n} submissions on {date}",
    prof_tabs_label: "Profile activity",
    prof_tab_recent: "Recent",
    prof_tab_solved: "Solved",
    prof_tab_posts: "Posts",
    prof_tab_all: "All submissions",
    prof_view_all: "View all submissions",
    prof_status_grading: "Grading",
    prof_status_failed: "Failed",
    prof_best_score: "Best {n}",
    prof_empty_submissions: "No submissions yet",
    prof_empty_solved: "No solved problems yet",
    prof_empty_posts: "You haven't posted in the forums yet",
    prof_empty_cta: "Find a problem",
    prof_empty_posts_cta: "Write your first post",
    home_hero_eyebrow: "Practice for AI-enabled interviews",
    home_hero_title: "Show how you think with AI.",
    home_hero_blurb: "Solve real engineering problems with an AI agent in the editor. Codritium scores more than the code you ship: it grades how you direct the agent, verify its work, and explain your decisions.",
    home_cta_create_account: "Create account",
    home_cta_continue: "Continue practicing",
    home_cta_browse: "Browse problems",
    home_stat_problems: "Hands-on problems",
    home_stat_tracks: "Problem tracks",
    home_stat_dimensions: "Scoring dimensions",
    home_mock_user_msg: "Why does a retried webhook ship twice?",
    home_mock_ai_msg: "Retries reuse the event id. I'd skip ids the ledger has already seen before dispatching.",
    home_mock_patch: "Proposed patch",
    home_mock_approve: "Approve",
    home_mock_reject: "Reject",
    home_mock_report: "Sample report",
    home_explore_title: "Start exploring",
    home_explore_blurb: "Everything you need to practice the way modern engineering interviews actually run.",
    home_feature_problems_title: "Real problems, not puzzles",
    home_feature_problems_desc: "Production-style incidents across debugging, security, and refactoring. Each one ships with starter code, tests, and a scenario you'd recognize from on-call.",
    home_feature_problems_link: "View problems",
    home_feature_agent_title: "An AI agent in your editor",
    home_feature_agent_desc: "Ask, delegate, and review. Every change the agent proposes arrives as a patch you approve, edit, or reject, so the decisions stay yours.",
    home_feature_agent_link: "Open a problem",
    home_feature_scoring_title: "Scored on five dimensions",
    home_feature_scoring_desc: "Correctness, problem decomposition, AI collaboration, verification, and communication are each graded on their own, with anti-pattern flags when you hand the wheel to the model.",
    home_feature_scoring_link: "Start a problem",
    home_visual_problem_count: "{n} problems",
    home_visual_patch_title: "Agent patch",
    home_visual_patch_hint: "Review before it lands",
    home_visual_patch_edit: "Edit",
    home_visual_antipatterns_clear: "No anti-patterns flagged",
    home_candidates_title: "Problems & community",
    home_candidates_desc: "Work through {n} hands-on problems, compare approaches in per-problem discussions, and trade notes with other candidates in the forums.",
    home_candidates_link_problems: "View problems",
    home_candidates_link_forums: "Visit the forums",
    home_teams_title: "For hiring teams",
    home_teams_desc: "Run AI-enabled technical assessments in the same workspace your candidates practice in.",
    home_teams_point_rubric: "A consistent five-dimension rubric for every candidate",
    home_teams_point_replay: "Code, prompt history, and a step-by-step replay of each session",
    home_teams_point_problems: "Realistic problems instead of algorithm trivia",
    home_teams_link: "Talk to our team",
    home_showcase_title: "Try a real problem",
    home_showcase_blurb: "Every problem comes with runnable starter code. Look through a few here, then open one in the full workspace with the AI agent beside you.",
    home_showcase_copy: "Copy",
    home_showcase_copied: "Copied",
    home_showcase_open: "Open in workspace",
    home_showcase_browse_all: "Browse all problems",
    home_mission_title: "Built for the AI-enabled interview",
    home_mission_body: "More teams now let candidates use AI during interviews. What they look for isn't typing speed. They want to see you break a problem down, direct an agent, check its work, and explain your call. Codritium exists to help you practice exactly that.",
    home_mission_skills_label: "What you'll practice",
    home_skill_incidents: "Debugging production incidents",
    home_skill_security: "Security review",
    home_skill_refactor: "Refactoring legacy code",
    home_skill_patches: "Reviewing AI patches",
    home_skill_tests: "Writing verification tests",
    home_skill_tradeoffs: "Explaining trade-offs",
    home_hiring_title: "Hiring engineers who work with AI?",
    home_hiring_body: "We'd love to help you run assessments that reflect how your team really builds software.",
    home_hiring_link: "Contact us",
    home_footer_copyright: "© {year} Codritium",
    home_footer_contact: "Contact",
    loading: "Loading…",
    submit: "Submit",
    cancel: "Cancel",
    close: "Close",
    restored_from_session: "Restored from your previous session.",
    problems_title: "Problems",
    problems_subtitle: "Practice AI-coding interview problems with an integrated agent.",
    problems_loading: "Loading problems…",
    open_a_file_first: "Open a file first.",
    chat_send_failed: "Could not send your message. Please try again.",
    chat_input_disabled_no_grader: "Chat is disabled — grader is unavailable.",
    ask_ai: "Ask AI…",
    ai_thinking: "AI is thinking…",
    ai_role: "AI",
    chat_subtitle: "Ask the AI about the problem. Selected code from the editor will be included automatically. Use code blocks; the \"Apply\" button inserts them into your file.",
    ask_tutor: "Ask the tutor…",
    tutor_thinking: "Tutor is thinking…",
    tutor_role: "Tutor",
    tutor_empty_state: "Stuck? Ask the tutor for a Socratic nudge. The tutor never writes code for you.",
    agent_tab_label: "Agent",
    tutor_tab_label: "Codritium Tips",
    tutor_submitted: "Submitted — review your decisions in the timeline",
    tutor_nudge_agent: "Stuck? Try the Tutor tab →",
    tutor_send_failed: "Could not reach the tutor. Please try again.",
    settings_link: "Settings",
    settings_title: "Settings",
    settings_subtitle: "Tune the workspace to your preferences. New options will land here as they become available.",
    settings_layout_section: "Layout",
    settings_layout_hint: "Choose how the Agent and Tutor surfaces are arranged. Side-by-side mode is planned for larger screens.",
    settings_font_section: "Editor font",
    settings_font_hint: "Adjust the Monaco editor font family and size used while practising.",
    settings_tips_visibility_section: "Codritium Tips visibility",
    settings_tips_visibility_hint: "Hide the Tutor tab if you prefer to practise without a Socratic helper.",
    settings_coming_soon: "Coming soon",
    submit_failed: "Submission failed. Your progress has been saved.",
    grading_failed: "Grading failed.",
    gemini_unavailable: "Grader is unavailable.",
    gemini_unavailable_progress_saved: "Grader is unavailable. Your progress has been saved locally.",
    submit_disabled_no_grader: "Submit is disabled — grader is unavailable.",
    login_title: "Sign in to Codritium",
    password_placeholder: "Password",
    password_required: "Password cannot be empty.",
    sign_in: "Sign in",
    sign_out: "Sign out",
    signing_in: "Signing in…",
    login_failed: "Could not sign in. Please try again.",
    register_card_title: "Create your account",
    register_card_desc: "Register with your email and a password to get started.",
    register_password_hint: "At least 8 characters",
    create_account: "Create account",
    register_prompt: "New to Codritium?",
    login_prompt: "Already have an account?",
    auth_email_exists: "An account with that email already exists. Sign in instead.",
    auth_invalid_credentials: "Email or password is incorrect.",
    auth_invalid_input: "Enter a valid email and a password of at least 8 characters.",
    auth_request_failed: "Could not reach the authentication service. Please try again.",
    session_quota_exceeded_oldest_evicted: "Local storage is full; the oldest saved session was removed.",
    apply_failed: "Could not apply the change. Please try again.",
    apply_in_progress: "Applying…",
    submissions_deleted: "Selected submissions deleted.",
    submissions_delete_partial: "Some submissions could not be deleted.",
    reply_page_title: "Your run · {slug}",
    reply_subtitle: "Step through your own AI session for this problem.",
    reply_back_link: "← Back to submission",
    reply_mode_tab: "Reply mode",
    answer_mode_tab: "Answer mode",
    reply_track_official: "Official",
    reply_track_mine: "My run",
    reply_empty_official_full: "No official walkthrough exists for this problem yet. Switch to Reply mode to view your own session, or check back after the team backfills it.",
    reply_no_official_short: "No official solution to show on the left yet. Backfill required.",
    reply_empty_mine: "No recorded run yet. Play a workspace session first.",
    reply_empty_official: "No official walkthrough yet.",
    diff_select_hint: "Select a file to see its diff.",
    diff_no_content: "No content for this file.",
    diff_select_from_left: "Select a file from the left to see its diff.",
    solution_explanation: "Solution explanation",
    explanation_empty: "No solution explanation yet for this problem.",
    filetree_files_header: "Files",
    filetree_modified_label: "modified",
    step_empty: "No envelopes to replay yet.",
    nav_prev: "Prev",
    nav_next: "Next",
    envelope_session_started: "Session started",
    envelope_session_submitted: "Session submitted",
    envelope_plan_in: "Plan mode · in",
    envelope_plan_out: "Plan mode · out",
    envelope_propose_tool: "Propose: tool use",
    envelope_propose_with_tool: "Propose: {tool}",
    envelope_tool_result: "Tool result",
    envelope_reasoning: "Reasoning",
    envelope_approved: "Approved",
    envelope_rejected: "Rejected",
    envelope_pushed_back: "Pushed back",
    envelope_tests_executed: "Tests executed",
    envelope_self_check: "Self-check artifact",
    envelope_reverted: "Reverted edit",
    role_engineer: "Engineer",
    dim_correctness_title: "Correctness",
    dim_correctness_desc: "Tests pass + edge cases honoured + implicit constraints preserved.",
    dim_decomposition_title: "Problem Decomposition",
    dim_decomposition_desc: "Plan-first prompts, sub-problem ordering, clarifying questions on ambiguity (Hard only).",
    dim_ai_collab_title: "AI Collaboration",
    dim_ai_collab_desc: "Course corrections, selective undo, architectural decisions kept by candidate.",
    dim_verification_title: "Verification & Quality",
    dim_verification_desc: "Tests run after generations, AI bugs caught, static analysis clean (Medium / Hard).",
    brand_name: "Codritium",
    appbar_readme: "README",
    appbar_help: "Help",
    chat_ai_assistant: "AI Assistant",
    chat_role_you: "You",
    chat_apply_btn: "Apply",
    chat_enter_to_queue_hint: "Enter to queue · Shift+Enter for newline",
    chat_enter_to_send_hint: "Enter to send · Shift+Enter for newline",
    chat_queue_btn: "Queue",
    chat_send_btn: "Send",
    chat_tab_enter: "Tab / Enter",
    chat_queued_prefix: "queued:",
    chat_proposed_word: "proposed",
    patch_label_auto: "Auto",
    patch_label_approved: "Approved",
    patch_label_modified: "Modified & approved",
    patch_label_rejected: "Rejected",
    patch_decision_failed: "Decision failed. Please try again.",
    patch_approve: "Approve",
    patch_modify: "Modify",
    patch_reject: "Reject",
    patch_reject_confirm: "Confirm reject",
    patch_reject_reason_placeholder: "Why are you rejecting? (optional — but feedback helps the AI iterate)",
    patch_save_approve: "Save & approve",
    patch_runcommand_warn: "Shell command — review before approving",
    pending_label: "Pending",
    login_card_title: "Sign in",
    login_card_desc: "Sign in with the email address and password you registered with.",
    login_account_label: "Email",
    login_password_placeholder_hint: "Password",
    login_footer_info: "Passwords are securely hashed. Your session lasts 30 days.",
    problem_tab_brief: "Brief",
    problem_tab_hint: "Hint",
    problem_tab_anti_patterns: "Anti-patterns",
    problem_tab_discussion: "Discussion",
    problem_tab_solutions: "Solutions",
    antipattern_hands_off_name: "Hands-off",
    antipattern_hands_off_desc: "Pasting an entire problem then accepting the first AI answer with no review.",
    antipattern_feature_marathon_name: "Feature marathon",
    antipattern_feature_marathon_desc: "Generating ten files of speculative code instead of fixing the actual bug.",
    antipattern_ai_showcase_name: "AI showcase",
    antipattern_ai_showcase_desc: "Letting the AI's framing dominate the prompt history without independent reasoning.",
    antipattern_not_thinking_name: "Not thinking",
    antipattern_not_thinking_desc: "No clarifying questions, no verification step, no test run before submitting.",
    agent_placeholder_system: "Agent scaffold: starter files, prompt history and grader feedback wired in v0.6+.",
    agent_placeholder_user: "Read cart.py and explain the floating-point bug in plain English.",
    agent_placeholder_agent: "cart_total accumulates `price * qty` in IEEE-754 doubles. Round-off propagates per item, so the final sum drifts off the 2-decimal expected value. Try rounding per line OR sum with Decimal then quantize.",
    streak_current_label: "Current streak",
    streak_active_today: "Active today.",
    streak_last_active_fmt: "Last active {days} day{s} ago.",
    no_activity_recorded: "No activity recorded.",
    streak_all_time_best: "All-time best.",
    tier_pro: "Pro",
    tier_free: "Free tier",
    pro_tier_desc: "Pro unlocks Sprint Interview content, full 5-dimension scoring, and submission trace archival.",
    public_history_title: "Public history",
    public_history_desc: "Submission heatmap + history will appear here when this user has published submissions with the \"Submission Heatmap\" privacy toggle on. v0.6+ scope.",
    browse_recent_forum_posts_prefix: "Browse recent forum posts by",
    this_user_via_forum: "this user via the forum",
    filter_ui_lands_v06: "(filter UI lands in v0.6+).",
    profile_category_all: "All",
    profile_category_debugging: "Debugging",
    profile_category_refactoring: "Refactoring",
    profile_category_feature_build: "Feature build",
    profile_category_security: "Security",
    profile_category_company_premium: "Company premium",
    forum_section_interview_label: "Interview",
    forum_section_career_label: "Career",
    forum_section_compensation_label: "Compensation",
    forum_section_feedback_label: "Feedback",
    forum_section_problems_label: "Problems",
    forum_for_you: "For You",
    forum_sections: "Forum sections",
    forum_create: "Create",
    forum_sort_votes: "Most Votes",
    forum_sort_newest: "Newest",
    forum_sort_best: "Best",
    forum_sort_comments: "Sort comments",
    forum_search_placeholder: "Search",
    forum_results_for_fmt: "Results for \"{q}\"",
    forum_clear_filter: "Clear filter",
    forum_trending: "Trending",
    forum_trending_empty: "Nothing trending yet. Start a discussion!",
    forum_pinned: "Pinned",
    forum_empty_feed: "No posts here yet. Be the first to start a discussion.",
    forum_load_more: "Load more",
    forum_load_more_comments: "Load more comments",
    forum_anonymous: "Anonymous",
    forum_verified: "Official",
    forum_posted_anonymously: "posted anonymously",
    forum_votes: "Votes",
    forum_views: "Views",
    forum_comments: "Comments",
    forum_comments_count_fmt: "Comments ({n})",
    forum_upvote: "Upvote",
    forum_downvote: "Downvote",
    forum_post_actions: "Post actions",
    forum_copy_link: "Copy link",
    forum_link_copied: "Link copied",
    forum_edit: "Edit",
    forum_delete: "Delete",
    forum_pin: "Pin to top",
    forum_unpin: "Unpin",
    forum_post_pinned: "Post pinned",
    forum_post_unpinned: "Post unpinned",
    forum_delete_post_confirm: "Delete this post? This can't be undone.",
    forum_post_deleted: "Post deleted",
    forum_edited: "edited",
    forum_reply: "Reply",
    forum_op_badge: "Author",
    forum_no_comments: "No comments yet. Start the conversation.",
    forum_comment_placeholder: "Share your thoughts… (Markdown supported)",
    forum_reply_placeholder: "Write a reply…",
    forum_post_comment: "Comment",
    forum_comment_anonymously: "Comment anonymously",
    forum_comment_deleted: "Comment deleted",
    forum_delete_comment_confirm: "Delete this comment? Its replies will be removed too.",
    forum_login_link: "Log in",
    forum_login_to_comment: "to join the discussion.",
    forum_new_post_title: "New post",
    forum_edit_post_title: "Edit post",
    forum_title_label: "Title",
    forum_title_placeholder: "Enter a title",
    forum_title_hint_fmt: "{n}/{max} characters (at least {min})",
    forum_section_label: "Section",
    forum_problem_label: "Related problem (optional)",
    forum_problem_none: "No specific problem",
    forum_tags_label_fmt: "Tags (up to {max})",
    forum_tags_placeholder: "Add a tag and press Enter",
    forum_remove_tag_fmt: "Remove tag {tag}",
    forum_write: "Write",
    forum_preview: "Preview",
    forum_markdown_hint: "Markdown: **bold**, `code`, ``` blocks, [links](https://…)",
    forum_body_label: "Post content",
    forum_body_placeholder: "Share your experience, question, or insight…",
    forum_preview_empty: "Nothing to preview yet.",
    forum_post_anonymously: "Post anonymously",
    forum_publish: "Post",
    forum_save: "Save changes",
    forum_post_published: "Post published",
    forum_post_updated: "Post updated",
    forum_err_login_required: "Please log in first.",
    forum_err_invalid_title: "Titles need 5–150 characters.",
    forum_err_invalid_body: "Write something before posting (up to 20,000 characters).",
    forum_err_invalid_tag: "Tags use letters, numbers, and - + # . (up to 24 characters).",
    forum_err_too_many_tags: "Use at most 5 tags.",
    forum_err_unknown_problem: "That problem doesn't exist.",
    forum_err_anonymous_not_allowed: "Anonymous posting is only available in Interview and Compensation.",
    forum_err_not_found: "This post or comment no longer exists.",
    forum_err_generic: "Something went wrong. Please try again.",
    forum_just_now: "just now",
    forum_minutes_ago_fmt: "{n}m ago",
    forum_hours_ago_fmt: "{n}h ago",
    forum_days_ago_fmt: "{n}d ago",
    submissions_empty_prefix: "No submissions yet. Start with the",
    submissions_problem_list_word: "problem list",
    submissions_total_fmt: "{total} total · page {page} of {pages}",
    submissions_delete_btn: "Delete",
    submissions_delete_btn_count_fmt: "Delete ({n})",
    submissions_select_btn: "Select",
    submissions_unknown_problem: "Unknown problem",
    submissions_select_row_aria_fmt: "Select submission for {title}",
    nav_problems: "Problems",
    nav_forum: "Forum",
    nav_forums: "Forums",
    nav_profile: "Profile",
    nav_notifications: "Notifications",
    menu_plans: "Plans",
    resume_just_now: "just now",
    resume_minute_ago: "1 minute ago",
    resume_minutes_ago_fmt: "{n} minutes ago",
    resume_hour_ago: "1 hour ago",
    resume_hours_ago_fmt: "{n} hours ago",
    resume_day_ago: "1 day ago",
    resume_days_ago_fmt: "{n} days ago",
    resume_start_solving_btn: "Start Solving",
    resume_start_fresh_btn: "Start fresh",
    resume_continue_btn: "Continue",
    resume_last_edited_fmt: "Last edited {when}",
    antipattern_label_hands_off: "Hands-Off",
    antipattern_label_feature_marathon: "Feature Marathon",
    antipattern_label_ai_showcase: "AI Showcase",
    antipattern_label_not_thinking: "Not Thinking",
    antipattern_flagged: "flagged",
    antipattern_clear: "clear",
    antipattern_not_evaluated: "not evaluated",
    problems_count_seed_fmt: "{count} problem{s} in the demo seed. Production v0.5 ships 12-15 problems across debugging, refactoring, feature build, and security.",
    problems_count_fmt: "{count} problem{s}",
    problems_filter_category: "Category",
    problems_filter_difficulty: "Difficulty",
    problems_filter_company: "Company",
    problems_filter_all: "All",
    problems_difficulty_easy: "Easy",
    problems_difficulty_medium: "Medium",
    problems_difficulty_hard: "Hard",
    problems_no_match: "No problems match the current filters.",
    problems_clear_filters: "Clear filters",
    problems_pro_lock_tooltip: "Requires Pro tier",
    problems_filter_status: "Status",
    problems_status_todo: "Todo",
    problems_status_attempted: "Attempted",
    problems_status_solved: "Solved",
    problems_col_title: "Title",
    problems_search_placeholder: "Search problems",
    problems_solved_count_fmt: "{count} solved",
    problems_page_indicator_fmt: "Page {page} of {total}",
    problems_page_prev: "Prev",
    problems_page_next: "Next",
    problems_page_word: "Page",
    problems_page_jump_aria: "Jump to page",
    submission_total_out_of_100: "out of 100",
    pro_badge: "Pro",
    breadcrumb_problems: "Problems",
    back_to_problems: "Back to problems",
    problem_intro_blurb: "Open the workspace to read the full task, edit files, talk to the AI assistant, and submit. Your in-progress edits are saved per-user across reload.",
    problem_tab_description: "Description",
    problem_tab_submissions: "Submissions",
    problem_solutions_unlocked_desc: "You've solved this problem. Compare your approach with the official walkthrough and community solutions.",
    problem_solutions_view_btn: "View solutions",
    problem_submissions_empty: "You haven't submitted this problem yet.",
    problem_submission_status_pending: "Pending",
    problem_submission_status_grading: "Grading",
    problem_submission_status_failed: "Grading failed",
    problem_start_title: "Ready to solve?",
    problem_start_point_files: "A multi-file workspace with the starter code",
    problem_start_point_ai: "An AI assistant and Codritium Tips beside the editor",
    problem_start_point_timer: "The timer starts when you open the workspace",
    problem_solutions_desc: "Top-rated community solutions unlock after you submit your own.",
    dim_label_correctness: "Correctness",
    dim_label_decomposition: "Problem Decomposition",
    dim_label_ai_collab: "AI Collaboration",
    dim_label_verification: "Verification & Quality",
    dim_label_communication: "Communication",
    submission_replay_btn: "Replay",
    submission_iterations_title: "Iterations",
    submission_resubmit_btn: "Resubmit",
    submission_not_graded_title: "Report not ready",
    submission_grading_body: "This submission is still being graded. The scored report appears here once grading finishes.",
    submission_failed_body: "Grading didn't complete for this submission. Resubmit from the problem page to try again.",
    submission_judge_prefix: "judge:",
    submission_meta_submitted_fmt: "Submitted {submitted}. Graded {graded}.",
    submission_five_dim_title: "Five-dimension breakdown",
    submission_five_dim_desc_prefix: "All difficulties use the same five weights. Dimensions marked \"—\" have insufficient evidence.",
    submission_code_grader_title: "Code-based grader",
    submission_code_grader_desc_fmt: "Pass rate {rate}%. In production this runs in an E2B Firecracker microVM; the demo uses deterministic mock test results.",
    submission_anti_pattern_title: "Anti-pattern flags",
    submission_anti_pattern_clean: "None of the four evaluated anti-patterns triggered.",
    submission_anti_pattern_count_fmt: "{n} of 4 patterns triggered.",
    submission_anti_pattern_unevaluated: "Insufficient evidence to evaluate anti-patterns.",
    submission_metadata_title: "Details",
    submission_meta_ai_tool: "AI tool",
    submission_meta_model: "Model",
    submission_meta_prompts: "Prompts",
    submission_meta_id: "Submission ID",
    submission_meta_lines_changed: "Lines changed",
    comments_title: "Discussion",
    comments_empty: "Be the first to share what worked.",
    comments_post_placeholder: "Share what you learned…",
    comments_post_btn: "Post",
    comments_load_more: "Load more",
    comments_reply_btn: "Reply",
    comments_delete_btn: "Delete",
    comments_must_complete: "Finish this problem at least once to join the discussion.",
    notes_title: "My notes",
    notes_empty: "No notes yet.",
    notes_create_placeholder: "Capture what you'd remind yourself next time…",
    notes_save_btn: "Save",
    notes_share_btn: "Share to discussion",
    notes_share_success: "Note shared as a comment.",
    notes_share_failed: "Could not share note. Try again.",
    reply_tab_label: "My run",
    report_tab_label: "Report",
    official_solution_title: "Official solution",
    official_solution_subtitle: "How an experienced engineer collaborates with AI to solve this problem.",
    official_locked_title: "Official solution locked",
    official_locked_body: "Submit your own attempt and get it graded to unlock the official walkthrough.",
    official_back_link: "← Back to problem",
    view_official_solution: "View official solution",
    reply_mode_official: "Official",
    reply_mode_user: "My run",
    reply_loading: "Loading reply…",
    tips_highlight_label: "Try the tutor",
    tips_first_visit_hint: "Hint: the tutor button is on the right.",
    readme_float_btn: "Float",
    readme_dock_btn: "Dock",
    readme_floating_title: "README is open in a floating window",
    readme_floating_hint: "Drag the window header to move it. Click Dock above to bring it back into this tab.",
    logout_confirm_title: "Sign out?",
    logout_confirm_body: "You will be returned to the sign-in page. Any unsaved work in this session is preserved per user.",
    logout_confirm_action: "Sign out",
    workspace_exit_title: "Leave workspace?",
    workspace_exit_body: "You'll return to the problem brief. Your in-progress edits stay saved for this problem.",
    workspace_exit_action: "Leave",
    workspace_exit_aria: "Back to problem brief",
    ide_split_btn: "Split",
    ide_close_pane_btn: "Close pane",
    ide_pane_limit_reached: "Pane limit reached.",
  },
  // zh: reserved for future locale-switch UI. Keep keys present but values
  // empty so t("key", { locale: "zh" }) doesn't crash; the switcher will
  // populate them in a dedicated i18n track.
  zh: {
    prof_member_since: "",
    prof_edit_btn: "",
    prof_edit_title: "",
    prof_edit_name: "",
    prof_edit_name_required: "",
    prof_edit_bio: "",
    prof_edit_bio_placeholder: "",
    prof_edit_region: "",
    prof_edit_region_placeholder: "",
    prof_edit_save: "",
    prof_edit_saving: "",
    prof_edit_saved: "",
    prof_edit_failed: "",
    prof_community_title: "",
    prof_stat_posts: "",
    prof_stat_comments: "",
    prof_stat_upvotes: "",
    prof_last_week: "",
    prof_tracks_title: "",
    prof_strengths_title: "",
    prof_strength_strong: "",
    prof_strength_developing: "",
    prof_strength_focus: "",
    prof_strength_samples: "",
    prof_not_enough_data: "",
    prof_solved_label: "",
    prof_attempting: "",
    prof_badges_title: "",
    prof_badge_next: "",
    prof_badge_locked: "",
    prof_badge_early: "",
    prof_badge_first: "",
    prof_badge_ten: "",
    prof_badge_streak: "",
    prof_badge_tracks: "",
    prof_badge_ninety: "",
    prof_badge_fifty: "",
    prof_activity_title: "",
    prof_active_days: "",
    prof_current_streak: "",
    prof_max_streak: "",
    prof_day_tooltip: "",
    prof_tabs_label: "",
    prof_tab_recent: "",
    prof_tab_solved: "",
    prof_tab_posts: "",
    prof_tab_all: "",
    prof_view_all: "",
    prof_status_grading: "",
    prof_status_failed: "",
    prof_best_score: "",
    prof_empty_submissions: "",
    prof_empty_solved: "",
    prof_empty_posts: "",
    prof_empty_cta: "",
    prof_empty_posts_cta: "",
    home_hero_eyebrow: "",
    home_hero_title: "",
    home_hero_blurb: "",
    home_cta_create_account: "",
    home_cta_continue: "",
    home_cta_browse: "",
    home_stat_problems: "",
    home_stat_tracks: "",
    home_stat_dimensions: "",
    home_mock_user_msg: "",
    home_mock_ai_msg: "",
    home_mock_patch: "",
    home_mock_approve: "",
    home_mock_reject: "",
    home_mock_report: "",
    home_explore_title: "",
    home_explore_blurb: "",
    home_feature_problems_title: "",
    home_feature_problems_desc: "",
    home_feature_problems_link: "",
    home_feature_agent_title: "",
    home_feature_agent_desc: "",
    home_feature_agent_link: "",
    home_feature_scoring_title: "",
    home_feature_scoring_desc: "",
    home_feature_scoring_link: "",
    home_visual_problem_count: "",
    home_visual_patch_title: "",
    home_visual_patch_hint: "",
    home_visual_patch_edit: "",
    home_visual_antipatterns_clear: "",
    home_candidates_title: "",
    home_candidates_desc: "",
    home_candidates_link_problems: "",
    home_candidates_link_forums: "",
    home_teams_title: "",
    home_teams_desc: "",
    home_teams_point_rubric: "",
    home_teams_point_replay: "",
    home_teams_point_problems: "",
    home_teams_link: "",
    home_showcase_title: "",
    home_showcase_blurb: "",
    home_showcase_copy: "",
    home_showcase_copied: "",
    home_showcase_open: "",
    home_showcase_browse_all: "",
    home_mission_title: "",
    home_mission_body: "",
    home_mission_skills_label: "",
    home_skill_incidents: "",
    home_skill_security: "",
    home_skill_refactor: "",
    home_skill_patches: "",
    home_skill_tests: "",
    home_skill_tradeoffs: "",
    home_hiring_title: "",
    home_hiring_body: "",
    home_hiring_link: "",
    home_footer_copyright: "",
    home_footer_contact: "",
    loading: "",
    submit: "",
    cancel: "",
    close: "",
    restored_from_session: "",
    problems_title: "",
    problems_subtitle: "",
    problems_loading: "",
    open_a_file_first: "",
    chat_send_failed: "",
    chat_input_disabled_no_grader: "",
    ask_ai: "",
    ai_thinking: "",
    ai_role: "",
    chat_subtitle: "",
    ask_tutor: "",
    tutor_thinking: "",
    tutor_role: "",
    tutor_empty_state: "",
    agent_tab_label: "",
    tutor_tab_label: "",
    tutor_submitted: "",
    tutor_nudge_agent: "",
    tutor_send_failed: "",
    settings_link: "",
    settings_title: "",
    settings_subtitle: "",
    settings_layout_section: "",
    settings_layout_hint: "",
    settings_font_section: "",
    settings_font_hint: "",
    settings_tips_visibility_section: "",
    settings_tips_visibility_hint: "",
    settings_coming_soon: "",
    submit_failed: "",
    grading_failed: "",
    gemini_unavailable: "",
    gemini_unavailable_progress_saved: "",
    submit_disabled_no_grader: "",
    login_title: "",
    password_placeholder: "",
    password_required: "",
    sign_in: "",
    sign_out: "",
    signing_in: "",
    login_failed: "",
    register_card_title: "",
    register_card_desc: "",
    register_password_hint: "",
    create_account: "",
    register_prompt: "",
    login_prompt: "",
    auth_email_exists: "",
    auth_invalid_credentials: "",
    auth_invalid_input: "",
    auth_request_failed: "",
    session_quota_exceeded_oldest_evicted: "",
    apply_failed: "",
    apply_in_progress: "",
    submissions_deleted: "",
    submissions_delete_partial: "",
    reply_page_title: "",
    reply_subtitle: "",
    reply_back_link: "",
    reply_mode_tab: "",
    answer_mode_tab: "",
    reply_track_official: "",
    reply_track_mine: "",
    reply_empty_official_full: "",
    reply_no_official_short: "",
    reply_empty_mine: "",
    reply_empty_official: "",
    diff_select_hint: "",
    diff_no_content: "",
    diff_select_from_left: "",
    solution_explanation: "",
    explanation_empty: "",
    filetree_files_header: "",
    filetree_modified_label: "",
    step_empty: "",
    nav_prev: "",
    nav_next: "",
    envelope_session_started: "",
    envelope_session_submitted: "",
    envelope_plan_in: "",
    envelope_plan_out: "",
    envelope_propose_tool: "",
    envelope_propose_with_tool: "",
    envelope_tool_result: "",
    envelope_reasoning: "",
    envelope_approved: "",
    envelope_rejected: "",
    envelope_pushed_back: "",
    envelope_tests_executed: "",
    envelope_self_check: "",
    envelope_reverted: "",
    role_engineer: "",
    dim_correctness_title: "",
    dim_correctness_desc: "",
    dim_decomposition_title: "",
    dim_decomposition_desc: "",
    dim_ai_collab_title: "",
    dim_ai_collab_desc: "",
    dim_verification_title: "",
    dim_verification_desc: "",
    brand_name: "",
    appbar_readme: "",
    appbar_help: "",
    chat_ai_assistant: "",
    chat_role_you: "",
    chat_apply_btn: "",
    chat_enter_to_queue_hint: "",
    chat_enter_to_send_hint: "",
    chat_queue_btn: "",
    chat_send_btn: "",
    chat_tab_enter: "",
    chat_queued_prefix: "",
    chat_proposed_word: "",
    patch_label_auto: "",
    patch_label_approved: "",
    patch_label_modified: "",
    patch_label_rejected: "",
    patch_decision_failed: "",
    patch_approve: "",
    patch_modify: "",
    patch_reject: "",
    patch_reject_confirm: "",
    patch_reject_reason_placeholder: "",
    patch_save_approve: "",
    patch_runcommand_warn: "",
    pending_label: "",
    login_card_title: "",
    login_card_desc: "",
    login_account_label: "",
    login_password_placeholder_hint: "",
    login_footer_info: "",
    problem_tab_brief: "",
    problem_tab_hint: "",
    problem_tab_anti_patterns: "",
    problem_tab_discussion: "",
    problem_tab_solutions: "",
    antipattern_hands_off_name: "",
    antipattern_hands_off_desc: "",
    antipattern_feature_marathon_name: "",
    antipattern_feature_marathon_desc: "",
    antipattern_ai_showcase_name: "",
    antipattern_ai_showcase_desc: "",
    antipattern_not_thinking_name: "",
    antipattern_not_thinking_desc: "",
    agent_placeholder_system: "",
    agent_placeholder_user: "",
    agent_placeholder_agent: "",
    streak_current_label: "",
    streak_active_today: "",
    streak_last_active_fmt: "",
    no_activity_recorded: "",
    streak_all_time_best: "",
    tier_pro: "",
    tier_free: "",
    pro_tier_desc: "",
    public_history_title: "",
    public_history_desc: "",
    browse_recent_forum_posts_prefix: "",
    this_user_via_forum: "",
    filter_ui_lands_v06: "",
    profile_category_all: "",
    profile_category_debugging: "",
    profile_category_refactoring: "",
    profile_category_feature_build: "",
    profile_category_security: "",
    profile_category_company_premium: "",
    forum_section_interview_label: "",
    forum_section_career_label: "",
    forum_section_compensation_label: "",
    forum_section_feedback_label: "",
    forum_section_problems_label: "",
    forum_for_you: "",
    forum_sections: "",
    forum_create: "",
    forum_sort_votes: "",
    forum_sort_newest: "",
    forum_sort_best: "",
    forum_sort_comments: "",
    forum_search_placeholder: "",
    forum_results_for_fmt: "",
    forum_clear_filter: "",
    forum_trending: "",
    forum_trending_empty: "",
    forum_pinned: "",
    forum_empty_feed: "",
    forum_load_more: "",
    forum_load_more_comments: "",
    forum_anonymous: "",
    forum_verified: "",
    forum_posted_anonymously: "",
    forum_votes: "",
    forum_views: "",
    forum_comments: "",
    forum_comments_count_fmt: "",
    forum_upvote: "",
    forum_downvote: "",
    forum_post_actions: "",
    forum_copy_link: "",
    forum_link_copied: "",
    forum_edit: "",
    forum_delete: "",
    forum_pin: "",
    forum_unpin: "",
    forum_post_pinned: "",
    forum_post_unpinned: "",
    forum_delete_post_confirm: "",
    forum_post_deleted: "",
    forum_edited: "",
    forum_reply: "",
    forum_op_badge: "",
    forum_no_comments: "",
    forum_comment_placeholder: "",
    forum_reply_placeholder: "",
    forum_post_comment: "",
    forum_comment_anonymously: "",
    forum_comment_deleted: "",
    forum_delete_comment_confirm: "",
    forum_login_link: "",
    forum_login_to_comment: "",
    forum_new_post_title: "",
    forum_edit_post_title: "",
    forum_title_label: "",
    forum_title_placeholder: "",
    forum_title_hint_fmt: "",
    forum_section_label: "",
    forum_problem_label: "",
    forum_problem_none: "",
    forum_tags_label_fmt: "",
    forum_tags_placeholder: "",
    forum_remove_tag_fmt: "",
    forum_write: "",
    forum_preview: "",
    forum_markdown_hint: "",
    forum_body_label: "",
    forum_body_placeholder: "",
    forum_preview_empty: "",
    forum_post_anonymously: "",
    forum_publish: "",
    forum_save: "",
    forum_post_published: "",
    forum_post_updated: "",
    forum_err_login_required: "",
    forum_err_invalid_title: "",
    forum_err_invalid_body: "",
    forum_err_invalid_tag: "",
    forum_err_too_many_tags: "",
    forum_err_unknown_problem: "",
    forum_err_anonymous_not_allowed: "",
    forum_err_not_found: "",
    forum_err_generic: "",
    forum_just_now: "",
    forum_minutes_ago_fmt: "",
    forum_hours_ago_fmt: "",
    forum_days_ago_fmt: "",
    submissions_empty_prefix: "",
    submissions_problem_list_word: "",
    submissions_total_fmt: "",
    submissions_delete_btn: "",
    submissions_delete_btn_count_fmt: "",
    submissions_select_btn: "",
    submissions_unknown_problem: "",
    submissions_select_row_aria_fmt: "",
    nav_problems: "",
    nav_forum: "",
    nav_forums: "",
    nav_profile: "",
    nav_notifications: "",
    menu_plans: "",
    resume_just_now: "",
    resume_minute_ago: "",
    resume_minutes_ago_fmt: "",
    resume_hour_ago: "",
    resume_hours_ago_fmt: "",
    resume_day_ago: "",
    resume_days_ago_fmt: "",
    resume_start_solving_btn: "",
    resume_start_fresh_btn: "",
    resume_continue_btn: "",
    resume_last_edited_fmt: "",
    antipattern_label_hands_off: "",
    antipattern_label_feature_marathon: "",
    antipattern_label_ai_showcase: "",
    antipattern_label_not_thinking: "",
    antipattern_flagged: "",
    antipattern_clear: "",
    antipattern_not_evaluated: "",
    problems_count_seed_fmt: "",
    problems_count_fmt: "",
    problems_filter_category: "",
    problems_filter_difficulty: "",
    problems_filter_company: "",
    problems_filter_all: "",
    problems_difficulty_easy: "",
    problems_difficulty_medium: "",
    problems_difficulty_hard: "",
    problems_no_match: "",
    problems_clear_filters: "",
    problems_pro_lock_tooltip: "",
    problems_filter_status: "",
    problems_status_todo: "",
    problems_status_attempted: "",
    problems_status_solved: "",
    problems_col_title: "",
    problems_search_placeholder: "",
    problems_solved_count_fmt: "",
    problems_page_indicator_fmt: "",
    problems_page_prev: "",
    problems_page_next: "",
    problems_page_word: "",
    problems_page_jump_aria: "",
    submission_total_out_of_100: "",
    pro_badge: "",
    breadcrumb_problems: "",
    back_to_problems: "",
    problem_intro_blurb: "",
    problem_tab_description: "",
    problem_tab_submissions: "",
    problem_solutions_unlocked_desc: "",
    problem_solutions_view_btn: "",
    problem_submissions_empty: "",
    problem_submission_status_pending: "",
    problem_submission_status_grading: "",
    problem_submission_status_failed: "",
    problem_start_title: "",
    problem_start_point_files: "",
    problem_start_point_ai: "",
    problem_start_point_timer: "",
    problem_solutions_desc: "",
    dim_label_correctness: "",
    dim_label_decomposition: "",
    dim_label_ai_collab: "",
    dim_label_verification: "",
    dim_label_communication: "",
    submission_replay_btn: "",
    submission_iterations_title: "",
    submission_resubmit_btn: "",
    submission_not_graded_title: "",
    submission_grading_body: "",
    submission_failed_body: "",
    submission_judge_prefix: "",
    submission_meta_submitted_fmt: "",
    submission_five_dim_title: "",
    submission_five_dim_desc_prefix: "",
    submission_code_grader_title: "",
    submission_code_grader_desc_fmt: "",
    submission_anti_pattern_title: "",
    submission_anti_pattern_clean: "",
    submission_anti_pattern_count_fmt: "",
    submission_anti_pattern_unevaluated: "",
    submission_metadata_title: "",
    submission_meta_ai_tool: "",
    submission_meta_model: "",
    submission_meta_prompts: "",
    submission_meta_id: "",
    submission_meta_lines_changed: "",
    comments_title: "",
    comments_empty: "",
    comments_post_placeholder: "",
    comments_post_btn: "",
    comments_load_more: "",
    comments_reply_btn: "",
    comments_delete_btn: "",
    comments_must_complete: "",
    notes_title: "",
    notes_empty: "",
    notes_create_placeholder: "",
    notes_save_btn: "",
    notes_share_btn: "",
    notes_share_success: "",
    notes_share_failed: "",
    reply_tab_label: "",
    report_tab_label: "",
    official_solution_title: "",
    official_solution_subtitle: "",
    official_locked_title: "",
    official_locked_body: "",
    official_back_link: "",
    view_official_solution: "",
    reply_mode_official: "",
    reply_mode_user: "",
    reply_loading: "",
    tips_highlight_label: "",
    tips_first_visit_hint: "",
    readme_float_btn: "",
    readme_dock_btn: "",
    readme_floating_title: "",
    readme_floating_hint: "",
    logout_confirm_title: "",
    logout_confirm_body: "",
    logout_confirm_action: "",
    workspace_exit_title: "",
    workspace_exit_body: "",
    workspace_exit_action: "",
    workspace_exit_aria: "",
    ide_split_btn: "",
    ide_close_pane_btn: "",
    ide_pane_limit_reached: "",
  },
};

// Default locale is en for MVP. The locale lives in a module-level external
// store so that:
//   - non-component callers (toast, session-store, ad-hoc helpers) can stay
//     on the synchronous t() API without touching React;
//   - the client-side `useLocale` hook (i18n-client.ts) subscribes via
//     useSyncExternalStore and re-renders on locale change.
// The store helpers are kept in this server-safe module so server components
// can still call t() during SSR without dragging React hooks across the
// Client / Server boundary.
let currentLocale: Locale = "en";
const subscribers = new Set<() => void>();

export function subscribe(fn: () => void): () => void {
  subscribers.add(fn);
  return () => {
    subscribers.delete(fn);
  };
}

export function getSnapshot(): Locale {
  return currentLocale;
}

export function getServerSnapshot(): Locale {
  // SSR always renders the default locale so the first client paint matches
  // the server-rendered HTML. Any saved preference is applied after hydration
  // via loadLocaleFromStorage().
  return "en";
}

export function setLocale(locale: Locale): void {
  if (currentLocale === locale) return;
  currentLocale = locale;
  if (typeof window !== "undefined") {
    try {
      window.localStorage.setItem("codritium-locale", locale);
    } catch {
      // localStorage may be unavailable (private mode, quota); the runtime
      // locale still updates so the UI flips this session.
    }
  }
  subscribers.forEach((fn) => fn());
}

// Pre-wired hydration helper. Not mounted by default per the MVP
// "no language switch UI" rule; a future LocaleHydrator client component
// can call this in an effect to honor a saved preference.
export function loadLocaleFromStorage(): void {
  if (typeof window === "undefined") return;
  try {
    const saved = window.localStorage.getItem("codritium-locale");
    if ((saved === "zh" || saved === "en") && saved !== currentLocale) {
      setLocale(saved);
    }
  } catch {
    // ignore — keep current locale on read failure
  }
}

// useLocale lives in i18n-client.ts ("use client") so server components can
// keep calling t() without pulling useSyncExternalStore through the
// Client / Server boundary.

// Lightweight {param} interpolation. Keeps t() synchronous and avoids
// pulling in a full ICU runtime. Falls back to en if the requested locale
// has an empty placeholder (the MVP default for zh).
export function t(
  key: LocaleKey,
  opts?: { locale?: Locale; params?: Record<string, string | number> },
): string {
  const locale = opts?.locale ?? currentLocale;
  const raw = STRINGS[locale]?.[key];
  const value = raw && raw.length > 0 ? raw : STRINGS.en[key] ?? key;
  const params = opts?.params;
  if (!params) return value;
  return Object.entries(params).reduce(
    (out, [k, v]) => out.replace(new RegExp(`\\{${k}\\}`, "g"), String(v)),
    value,
  );
}

// Re-export the key union type so other modules can type-check their toast keys.
export type { LocaleKey, Locale };
