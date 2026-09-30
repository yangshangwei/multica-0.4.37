import { describe, it, expect, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { I18nProvider } from "@multica/core/i18n/react";
import { createAuthStore, registerAuthStore } from "@multica/core/auth";
import { ApiClient, setApiInstance } from "@multica/core/api";
import { configStore } from "@multica/core/config";
import type { User } from "@multica/core/types";
const EMPTY_USER = {onboarded_at:null} as User;
import en from "../locales/en/auth.json";
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
    await user.type(screen.getByLabelText('Username'),'alice');
    await user.type(screen.getByLabelText('Name'),'Alice');
    await user.type(screen.getByLabelText('Password'),'a long password');
    expect(screen.getByLabelText('Password').getAttribute('autocomplete')).toBe('new-password');
    await user.click(screen.getByRole('button',{name:'Show password'}));
    expect(screen.getByLabelText('Password').getAttribute('type')).toBe('text');
    await user.click(screen.getByRole('button',{name:'Register and get started'}));
    await waitFor(() => expect(done).toHaveBeenCalledOnce());
    expect(register).toHaveBeenCalledWith('alice','a long password','Alice');
    expect(store.getState().user?.onboarded_at).toBeNull();
  });
});
