# Copyright 2018 ICON Foundation
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

from typing import Any
from iconcommons import Logger

from .base.address import Address, GETAPI_DUMMY_ADDRESS
from .base.exception import ServerErrorException, IconServiceBaseException, ExceptionCode
from .base.type_converter import TypeConverter
from .database.factory import ContextDatabaseFactory

from .icon_constant import Status
from .iconscore.icon_score_base import IconScoreBase, ScoreErrorException, InvalidParamsException
from .iconscore.icon_score_context import ContextContainer, IconScoreContext
from .iconscore.icon_score_eventlog import EventLogEmitter
from .iconscore.icon_score_mapper import IconScoreMapper
from .iconscore.icon_score_step import StepType
from .iconscore.internal_call import InternalCall

TAG = 'ServiceEngine'


def decode_params(values: dict) -> dict:
    result = {}
    if isinstance(values, dict):
        for k, v in values.items():
            new_key = k
            if isinstance(k, bytes):
                new_key = k.decode()
            elif not isinstance(k, str):
                raise BaseException('Unexpected key type')

            if isinstance(v, bytes):
                result[new_key] = v.decode()
            else:
                result[new_key] = v
    return result


class ServiceEngine(ContextContainer):

    _score_mapper: None

    @classmethod
    def open(cls, proxy):
        cls._score_mapper = IconScoreMapper()
        ContextDatabaseFactory.open(proxy, ContextDatabaseFactory.Mode.SINGLE_DB)
        EventLogEmitter.open(proxy)
        InternalCall.open(proxy)

    @classmethod
    def invoke(cls, context: IconScoreContext):
        Logger.info(f'[invoke] {context.method}, {context.params}', TAG)

        cls._push_context(context)
        status, step_used, ret = cls._handle_invoke(context)
        cls._pop_context()

        print(f'*** RESULT: {status}, {step_used}, {ret}')
        return status, step_used, ret

    @classmethod
    def get_score_api(cls, code: str):
        icon_score: 'IconScoreBase' = cls._get_icon_score(GETAPI_DUMMY_ADDRESS, code)
        return icon_score.get_api()

    @classmethod
    def _get_icon_score(cls, address: Address, code: str):
        return cls._score_mapper.get_icon_score(address, code)

    @classmethod
    def _handle_invoke(cls, context):
        try:
            ret = cls._internal_call(context)
            status = Status.SUCCESS
        except BaseException as e:
            ret = cls._get_failure_from_exception(e)
            status = Status.FAILURE
        finally:
            step_used = context.step_counter.step_used

        return status, step_used, ret

    @classmethod
    def _internal_call(cls, context: IconScoreContext):
        icon_score: 'IconScoreBase' = cls._get_icon_score(context.to, context.code)
        if icon_score is None:
            raise ServerErrorException(
                f'SCORE not found: {context.to}')

        func_name: str = context.method
        context.set_func_type_by_icon_score(icon_score, func_name)

        if isinstance(context.params, dict):
            arg_params = []
            params: dict = decode_params(context.params)
            kw_params = cls._convert_score_params_by_annotations(icon_score, func_name, params)
            Logger.info(f'-- kw_params: {kw_params}', TAG)
        elif isinstance(context.params, list):
            arg_params: list = context.params
            Logger.info(f'-- arg_params: {arg_params}', TAG)
            kw_params = {}
        else:
            raise InvalidParamsException('Unknown params type')

        context.step_counter.apply_step(StepType.CONTRACT_CALL, 1)

        score_func = getattr(icon_score, '_IconScoreBase__call')
        return score_func(func_name=func_name, arg_params=arg_params, kw_params=kw_params)

    @staticmethod
    def _convert_score_params_by_annotations(icon_score: 'IconScoreBase', func_name: str, kw_params: dict) -> dict:
        tmp_params = kw_params
        score_func = getattr(icon_score, func_name)
        annotation_params = TypeConverter.make_annotations_from_method(score_func)
        TypeConverter.convert_data_params(annotation_params, tmp_params)
        return tmp_params

    @classmethod
    def _get_failure_from_exception(cls, e: BaseException):
        if isinstance(e, IconServiceBaseException):
            if e.code == ExceptionCode.SCORE_ERROR or isinstance(e, ScoreErrorException):
                Logger.warning(e.message, TAG)
            else:
                Logger.exception(e.message, TAG)

            code = e.code
            message = e.message
        else:
            Logger.exception(e, TAG)
            Logger.error(e, TAG)

            code = ExceptionCode.SERVER_ERROR
            message = str(e)

        return cls._make_error_response(code, message)

    @staticmethod
    def _make_error_response(code: Any, message: str):
        return {'error': {'code': int(code), 'message': message}}
