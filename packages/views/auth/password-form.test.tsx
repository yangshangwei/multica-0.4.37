import { describe, it, expect, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { I18nProvider } from "@multica/core/i18n/react";
import { createAuthStore, registerAuthStore } from "@multica/core/auth";
import { ApiClient, ApiError, setApiInstance } from "@multica/core/api";
import { configStore } from "@multica/core/config";
import type { User } from "@multica/core/types";
const EMPTY_USER = {onboarded_at:null} as User;
import en from "../locales/en/auth.json";
import zh from "../locales/zh-Hans/auth.json";
import { PasswordForm } from "./password-form";

describe('password form', () => {
  it('registers with three labelled fields, supports password visibility, and preserves onboarding', async () => {
    const api = new ApiClient('http://localhost');
    const register = vi.spyOn(api,'registerPassword').mockResolvedValue({token:'session',user:{...EMPTY_USER,id:'u1'}});
    vi.spyOn(api,'listWorkspaces').mockResolvedValue([]);
    setApiInstance(api);
    const store = createAuthStore({api,storage:{getItem:()=>null,setItem:vi.fn(),removeItem:vi.fn()}});
    registerAuthStore(store);
    configStore.getState().setAuthConfig({allowSignup:true,passwordSignupAvailable:true});
    const done = vi.fn();
    render(<I18nProvider locale="en" resources={{en:{auth:en}}}><QueryClientProvider client={new QueryClient()}><PasswordForm onSuccess={done}/></QueryClientProvider></I18nProvider>);
    const user = userEvent.setup();
    await user.click(screen.getByRole('button',{name:'Create account'}));
    expect(screen.getByLabelText('Username')).toHaveAccessibleDescription(/employee ID.*numbers only/i);
    await user.type(screen.getByLabelText('Username'),'0011052');
    await user.type(screen.getByLabelText('Name'),'Alice');
    expect(screen.getByLabelText('Password')).toHaveAccessibleDescription('Use 6–128 characters. Spaces and pasting are supported.');
    await user.type(screen.getByLabelText('Password'),'abc123');
    expect(screen.getByLabelText('Password').getAttribute('autocomplete')).toBe('new-password');
    await user.click(screen.getByRole('button',{name:'Show password'}));
    expect(screen.getByLabelText('Password').getAttribute('type')).toBe('text');
    await user.click(screen.getByRole('button',{name:'Register and get started'}));
    await waitFor(() => expect(done).toHaveBeenCalledOnce());
    expect(register).toHaveBeenCalledWith('0011052','abc123','Alice');
    expect(store.getState().user?.onboarded_at).toBeNull();
  });

  it('completes admin login without requesting workspaces', async () => {
    const api = new ApiClient('http://localhost');
    vi.spyOn(api, 'passwordLogin').mockResolvedValue({token: 'session', user: {...EMPTY_USER, id: 'admin'}});
    const list = vi.spyOn(api, 'listWorkspaces').mockRejectedValue(new Error('Workspace service unavailable'));
    setApiInstance(api);
    registerAuthStore(createAuthStore({api, storage: {getItem: () => null, setItem: vi.fn(), removeItem: vi.fn()}}));
    configStore.getState().setAuthConfig({allowSignup: false, passwordSignupAvailable: false});
    const done = vi.fn();
    render(<I18nProvider locale="en" resources={{en: {auth: en}}}><QueryClientProvider client={new QueryClient()}><PasswordForm skipWorkspaceBootstrap onSuccess={done}/></QueryClientProvider></I18nProvider>);
    const user = userEvent.setup();
    await user.type(screen.getByLabelText('Username'), 'admin');
    await user.type(screen.getByLabelText('Password'), 'secret123');
    await user.click(screen.getByRole('button', {name: 'Sign in'}));
    await waitFor(() => expect(done).toHaveBeenCalledOnce());
    expect(list).not.toHaveBeenCalled();
  });

  it.each([
    ['invalid_username', 'username must contain 3–32 ASCII characters', '用户名需为 3–32 位英文字母、数字或下划线，支持纯数字。'],
    ['invalid_name', 'name must contain 1–80 characters', '姓名需为 1–80 个字符。'],
    ['invalid_password', 'password must contain 6–128 characters and at most 512 bytes', '密码需为 6–128 个字符，支持空格和粘贴。'],
    ['invalid_request', 'username must start with a letter and contain only letters, numbers or underscores', '此服务器仍使用旧的用户名规则，暂不支持数字或下划线开头。请联系部署维护人员更新服务器。'],
    ['signup_disabled', 'Registration is disabled', '此服务器暂未开放注册，请联系部署维护人员。'],
    ['auth_unavailable', 'Authentication service unavailable', '账号服务暂时不可用，请稍后再试。'],
    ['unknown_code', 'Internal server error', '暂时无法完成操作，请稍后重试。'],
  ])('localizes registration error %s in Chinese', async (code, message, expected) => {
    const api = new ApiClient('http://localhost');
    vi.spyOn(api, 'registerPassword').mockRejectedValue(new ApiError(message, 400, 'Bad Request', {code, error: message}));
    setApiInstance(api);
    registerAuthStore(createAuthStore({api, storage: {getItem: () => null, setItem: vi.fn(), removeItem: vi.fn()}}));
    configStore.getState().setAuthConfig({allowSignup: true, passwordSignupAvailable: true});
    const done = vi.fn();
    render(<I18nProvider locale="zh-Hans" resources={{'zh-Hans': {auth: zh}}}><QueryClientProvider client={new QueryClient()}><PasswordForm onSuccess={done}/></QueryClientProvider></I18nProvider>);
    const user = userEvent.setup();
    await user.click(screen.getByRole('button', {name: '创建账号'}));
    await user.type(screen.getByLabelText('用户名'), '11052');
    await user.type(screen.getByLabelText('姓名'), 'Test User');
    await user.type(screen.getByLabelText('密码'), 'a long password');
    await user.click(screen.getByRole('button', {name: '注册并开始使用'}));
    expect(await screen.findByRole('alert')).toHaveTextContent(expected);
    expect(screen.getByRole('alert')).not.toHaveTextContent(message);
    expect(done).not.toHaveBeenCalled();
  });

  // The detailed error matrix belongs in password-error.test.ts; keep DOM wiring here.
  it('shows the required and actual password lengths for an older backend response', async () => {
    const api = new ApiClient('http://localhost');
    const message = 'password must contain 12–128 characters and at most 512 bytes';
    vi.spyOn(api, 'registerPassword').mockRejectedValue(new ApiError(message, 400, 'Bad Request', {code: 'invalid_request', error: message}));
    setApiInstance(api);
    registerAuthStore(createAuthStore({api, storage: {getItem: () => null, setItem: vi.fn(), removeItem: vi.fn()}}));
    configStore.getState().setAuthConfig({allowSignup: true, passwordSignupAvailable: true});
    render(<I18nProvider locale="zh-Hans" resources={{'zh-Hans': {auth: zh}}}><QueryClientProvider client={new QueryClient()}><PasswordForm onSuccess={vi.fn()}/></QueryClientProvider></I18nProvider>);
    const user = userEvent.setup();
    await user.click(screen.getByRole('button', {name: '创建账号'}));
    await user.type(screen.getByLabelText('用户名'), '11052');
    await user.type(screen.getByLabelText('姓名'), 'Test User');
    expect(screen.getByLabelText('密码')).toHaveAccessibleDescription('使用 6–128 个字符，支持空格和粘贴。');
    await user.type(screen.getByLabelText('密码'), 'short');
    await user.click(screen.getByRole('button', {name: '注册并开始使用'}));
    expect(await screen.findByRole('alert')).toHaveTextContent('密码至少需要 6 个字符，当前为 5 个。');
    expect(screen.getByLabelText('密码')).toHaveValue('short');
  });
});
