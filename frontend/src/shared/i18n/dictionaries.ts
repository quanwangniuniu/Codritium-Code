// The one place that knows every feature's dictionary. Feature folders own
// their strings (features/<f>/i18n.ts); this file merges them so LocaleKey
// can be derived from the English source. Dictionaries are plain data, so
// importing them here pulls no feature code into shared/.

import { authEn } from "@/features/auth/i18n";
import { forumEn } from "@/features/forum/i18n";
import { homeEn } from "@/features/home/i18n";
import { problemsEn } from "@/features/problems/i18n";
import { profileEn } from "@/features/profile/i18n";
import { replyEn } from "@/features/reply/i18n";
import { settingsEn } from "@/features/settings/i18n";
import { submissionsEn } from "@/features/submissions/i18n";
import { workspaceEn } from "@/features/workspace/i18n";
import { commonEn } from "./common";

export const EN_DICTIONARIES = {
  common: commonEn,
  auth: authEn,
  forum: forumEn,
  home: homeEn,
  problems: problemsEn,
  profile: profileEn,
  reply: replyEn,
  settings: settingsEn,
  submissions: submissionsEn,
  workspace: workspaceEn,
};

export const en = {
  ...commonEn,
  ...authEn,
  ...forumEn,
  ...homeEn,
  ...problemsEn,
  ...profileEn,
  ...replyEn,
  ...settingsEn,
  ...submissionsEn,
  ...workspaceEn,
};

export type LocaleKey = keyof typeof en;
