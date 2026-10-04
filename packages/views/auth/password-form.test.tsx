import { describe, it, expect, vi } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
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
  it('registers with matching passwords, supports visibility, and preserves onboarding', async () => {
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
    await user.type(screen.getByLabelText('Password'),' abc123 ');
    await user.type(screen.getByLabelText('Confirm password'),' abc123 ');
    expect(screen.getByLabelText('Password').getAttribute('autocomplete')).toBe('new-password');
    expect(screen.getByLabelText('Confirm password')).toHaveAttribute('autocomplete', 'new-password');
    expect(screen.getByLabelText('Confirm password')).toHaveAttribute('type', 'password');
    const passwordToggle = screen.getByRole('button', {name: 'Show password', description: 'Password'});
    const confirmationToggle = screen.getByRole('button', {name: 'Show password', description: 'Confirm password'});
    expect(passwordToggle).toHaveAttribute('aria-controls', 'password-value');
    expect(confirmationToggle).toHaveAttribute('aria-controls', 'password-confirm');
    await user.click(passwordToggle);
    expect(screen.getByLabelText('Password').getAttribute('type')).toBe('text');
    expect(screen.getByLabelText('Confirm password')).toHaveAttribute('type', 'password');
    await user.click(screen.getByLabelText('Confirm password'));
    await user.tab();
    expect(confirmationToggle).toHaveFocus();
    await user.keyboard(' ');
    expect(screen.getByLabelText('Confirm password')).toHaveAttribute('type', 'text');
    expect(confirmationToggle).toHaveAttribute('aria-pressed', 'true');
    await user.click(screen.getByRole('button', {name: 'Hide password', description: 'Password'}));
    expect(screen.getByLabelText('Password')).toHaveAttribute('type', 'password');
    expect(screen.getByLabelText('Confirm password')).toHaveAttribute('type', 'text');
    await user.click(screen.getByRole('button', {name: 'Hide password', description: 'Confirm password'}));
    expect(screen.getByLabelText('Confirm password')).toHaveAttribute('type', 'password');
    expect(register).not.toHaveBeenCalled();
    await user.click(screen.getByRole('button',{name:'Register and get started'}));
    await waitFor(() => expect(done).toHaveBeenCalledOnce());
    expect(register).toHaveBeenCalledExactlyOnceWith('0011052',' abc123 ','Alice');
    expect(screen.getByLabelText('Password')).toHaveValue('');
    expect(screen.getByLabelText('Confirm password')).toHaveValue('');
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    expect(store.getState().user?.onboarded_at).toBeNull();
  });

  it('requires password confirmation before registering', async () => {
    const api = new ApiClient('http://localhost');
    const register = vi.spyOn(api, 'registerPassword').mockResolvedValue({token: 'session', user: {...EMPTY_USER, id: 'u1'}});
    setApiInstance(api);
    registerAuthStore(createAuthStore({api, storage: {getItem: () => null, setItem: vi.fn(), removeItem: vi.fn()}}));
    configStore.getState().setAuthConfig({allowSignup: true, passwordSignupAvailable: true});
    const done = vi.fn();
    render(<I18nProvider locale="en" resources={{en: {auth: en}}}><QueryClientProvider client={new QueryClient()}><PasswordForm onSuccess={done}/></QueryClientProvider></I18nProvider>);
    const user = userEvent.setup();
    await user.click(screen.getByRole('button', {name: 'Create account'}));
    await user.type(screen.getByLabelText('Username'), '11052');
    await user.type(screen.getByLabelText('Name'), 'Alice');
    await user.type(screen.getByLabelText('Password'), 'abc123');
    expect(screen.getByLabelText('Confirm password')).toBeRequired();
    await user.click(screen.getByRole('button', {name: 'Register and get started'}));
    expect(screen.getByLabelText('Confirm password')).toBeInvalid();
    expect(register).not.toHaveBeenCalled();
    expect(done).not.toHaveBeenCalled();
  });

  it('blocks mismatches, rechecks either password, and clears the Chinese error when corrected', async () => {
    const api = new ApiClient('http://localhost');
    const register = vi.spyOn(api, 'registerPassword').mockResolvedValue({token: 'session', user: {...EMPTY_USER, id: 'u1'}});
    vi.spyOn(api, 'listWorkspaces').mockResolvedValue([]);
    setApiInstance(api);
    registerAuthStore(createAuthStore({api, storage: {getItem: () => null, setItem: vi.fn(), removeItem: vi.fn()}}));
    configStore.getState().setAuthConfig({allowSignup: true, passwordSignupAvailable: true});
    const done = vi.fn();
    render(<I18nProvider locale="zh-Hans" resources={{'zh-Hans': {auth: zh}}}><QueryClientProvider client={new QueryClient()}><PasswordForm onSuccess={done}/></QueryClientProvider></I18nProvider>);
    const user = userEvent.setup();
    await user.click(screen.getByRole('button', {name: '创建账号'}));
    await user.type(screen.getByLabelText('用户名'), '11052');
    await user.type(screen.getByLabelText('姓名'), 'Test User');
    await user.type(screen.getByLabelText('密码'), ' abc123 ');
    const confirmation = screen.getByLabelText('确认密码');
    await user.type(confirmation, 'abc123');
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    // user-event does not find submit buttons outside the form; Enter is checked in the browser.
    fireEvent.submit(confirmation.closest('form')!);
    expect(screen.getByRole('alert')).toHaveTextContent('两次输入的密码不一致，请重新输入。');
    expect(confirmation).toHaveAccessibleDescription('两次输入的密码不一致，请重新输入。');
    expect(confirmation).toHaveAttribute('aria-invalid', 'true');
    expect(confirmation).toHaveFocus();
    expect(register).not.toHaveBeenCalled();
    expect(done).not.toHaveBeenCalled();

    await user.clear(confirmation);
    await user.type(confirmation, ' abc123 ');
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    expect(confirmation).not.toHaveAttribute('aria-invalid', 'true');
    await user.type(screen.getByLabelText('密码'), '4');
    expect(screen.getByRole('alert')).toHaveTextContent('两次输入的密码不一致，请重新输入。');
    await user.click(screen.getByRole('button', {name: '注册并开始使用'}));
    expect(register).not.toHaveBeenCalled();
    expect(confirmation).toHaveFocus();
    await user.type(confirmation, '4');
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    await user.click(screen.getByRole('button', {name: '注册并开始使用'}));
    await waitFor(() => expect(done).toHaveBeenCalledOnce());
    expect(register).toHaveBeenCalledExactlyOnceWith('11052', ' abc123 4', 'Test User');
  });

  it('clears confirmation and blur validation when switching back to registration', async () => {
    const api = new ApiClient('http://localhost');
    setApiInstance(api);
    registerAuthStore(createAuthStore({api, storage: {getItem: () => null, setItem: vi.fn(), removeItem: vi.fn()}}));
    configStore.getState().setAuthConfig({allowSignup: true, passwordSignupAvailable: true});
    render(<I18nProvider locale="en" resources={{en: {auth: en}}}><QueryClientProvider client={new QueryClient()}><PasswordForm onSuccess={vi.fn()}/></QueryClientProvider></I18nProvider>);
    const user = userEvent.setup();
    expect(screen.queryByLabelText('Confirm password')).not.toBeInTheDocument();
    await user.click(screen.getByRole('button', {name: 'Create account'}));
    await user.type(screen.getByLabelText('Password'), 'abc123');
    await user.type(screen.getByLabelText('Confirm password'), 'different');
    await user.tab();
    expect(screen.getByRole('alert')).toHaveTextContent('Passwords do not match. Please try again.');
    await user.click(screen.getByRole('button', {name: 'Show password', description: 'Password'}));
    await user.click(screen.getByRole('button', {name: 'Already have an account? Sign in'}));
    expect(screen.getByLabelText('Password')).toHaveAttribute('type', 'password');
    expect(screen.queryByLabelText('Confirm password')).not.toBeInTheDocument();
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    await user.click(screen.getByRole('button', {name: 'Create account'}));
    expect(screen.getByLabelText('Confirm password')).toHaveValue('');
    await user.type(screen.getByLabelText('Password'), 'abc123');
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });

  it('toggles current and new passwords independently when changing a password', async () => {
    const api = new ApiClient('http://localhost');
    setApiInstance(api);
    registerAuthStore(createAuthStore({api, storage: {getItem: () => null, setItem: vi.fn(), removeItem: vi.fn()}}));
    render(<I18nProvider locale="en" resources={{en: {auth: en}}}><QueryClientProvider client={new QueryClient()}><PasswordForm mode="change" onSuccess={vi.fn()}/></QueryClientProvider></I18nProvider>);
    const user = userEvent.setup();
    await user.type(screen.getByLabelText('Current password'), 'old123');
    await user.type(screen.getByLabelText('New password'), 'new123');
    await user.click(screen.getByRole('button', {name: 'Show password', description: 'Current password'}));
    expect(screen.getByLabelText('Current password')).toHaveAttribute('type', 'text');
    expect(screen.getByLabelText('New password')).toHaveAttribute('type', 'password');
    await user.click(screen.getByRole('button', {name: 'Show password', description: 'New password'}));
    expect(screen.getByLabelText('New password')).toHaveAttribute('type', 'text');
    await user.click(screen.getByRole('button', {name: 'Hide password', description: 'Current password'}));
    expect(screen.getByLabelText('Current password')).toHaveAttribute('type', 'password');
    expect(screen.getByLabelText('New password')).toHaveValue('new123');
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
    expect(screen.queryByLabelText('Confirm password')).not.toBeInTheDocument();
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
    await user.type(screen.getByLabelText('确认密码'), 'a long password');
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
    await user.type(screen.getByLabelText('确认密码'), 'short');
    await user.click(screen.getByRole('button', {name: '注册并开始使用'}));
    expect(await screen.findByRole('alert')).toHaveTextContent('密码至少需要 6 个字符，当前为 5 个。');
    expect(screen.getByLabelText('密码')).toHaveValue('short');
  });
});
