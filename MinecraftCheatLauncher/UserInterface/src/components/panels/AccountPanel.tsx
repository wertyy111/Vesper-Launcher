import { photinoBridge } from '../../bridge';
import type { PanelRenderProps } from '../../types';
import { PanelHeader } from '../common/PanelHeader';

export function AccountPanel({ launcher, accountForm, setAccountForm, setAccountDirty, submitAccount }: PanelRenderProps) {
  const account = launcher.account;
  const recentUsernames = (account.recentUsernames ?? []) as string[];

  return (
    <>
      <PanelHeader title="Аккаунт Vesper" subtitle={account.accountStateText} />

      <div className="avatar-summary account-avatar-summary">
        {account.avatarUrl ? <img className="avatar-preview" src={account.avatarUrl} alt="avatar" /> : <div className="avatar-placeholder hero">{account.avatarPlaceholder}</div>}
        <div>
          <h3>{account.currentNickname || 'Без аккаунта'}</h3>
          <p>{account.hasAuthenticatedSession ? 'Облачная сессия активна.' : account.hasGuestIdentity ? 'Режим инкогнито.' : 'Сессия не активна.'}</p>
        </div>
      </div>

      {account.mode === 'summary' ? (
        <div className="stack-layout">
          <button className="primary-button" onClick={() => photinoBridge.sendCommand('account.pickAvatar')} type="button">{account.canChangeAvatar ? 'Сменить аватар' : 'Изменить профиль'}</button>
          <button className="danger-button" onClick={() => photinoBridge.sendCommand('account.logout')} type="button">Выйти</button>
          <div className="chip-group">
            {recentUsernames.map((username) => (
              <button key={username} className="chip-button" onClick={() => photinoBridge.sendCommand('account.selectRecentUsername', { username })} type="button">{username}</button>
            ))}
          </div>
        </div>
      ) : (
        <div className="stack-layout">
          <div className="tab-row compact">
            {['login', 'register', 'guest'].map((mode) => (
              <button
                key={mode}
                className={`tab-button ${accountForm.mode === mode ? 'active' : ''}`}
                onClick={() => {
                  setAccountDirty(false);
                  setAccountForm((previous: any) => ({ ...previous, mode, password: '' }));
                  photinoBridge.sendCommand('account.setMode', { mode });
                }}
                type="button"
              >
                {mode === 'login' ? 'Вход' : mode === 'register' ? 'Регистрация' : 'Инкогнито'}
              </button>
            ))}
          </div>

          <label className="field-label">
            Ник
            <input
              className="launcher-input"
              value={accountForm.username}
              onChange={(event) => {
                setAccountDirty(true);
                setAccountForm((previous: any) => ({ ...previous, username: event.target.value }));
              }}
              placeholder="Введите ник"
            />
          </label>

          {accountForm.mode !== 'guest' ? (
            <label className="field-label">
              Пароль
              <input
                className="launcher-input"
                type="password"
                value={accountForm.password}
                onChange={(event) => {
                  setAccountDirty(true);
                  setAccountForm((previous: any) => ({ ...previous, password: event.target.value }));
                }}
                placeholder="Введите пароль"
              />
            </label>
          ) : null}

          <button className="primary-button" onClick={submitAccount} type="button">{accountForm.mode === 'register' ? 'Создать аккаунт' : accountForm.mode === 'guest' ? 'Играть без аккаунта' : 'Войти'}</button>

          <div className="chip-group">
            {recentUsernames.map((username) => (
              <button
                key={username}
                className="chip-button"
                onClick={() => {
                  setAccountDirty(true);
                  setAccountForm((previous: any) => ({ ...previous, username }));
                }}
                type="button"
              >
                {username}
              </button>
            ))}
          </div>
        </div>
      )}
    </>
  );
}
