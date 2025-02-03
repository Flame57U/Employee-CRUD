package org.example.demo.service;


import com.baomidou.mybatisplus.core.metadata.IPage;
import com.baomidou.mybatisplus.extension.service.IService;
import org.example.demo.model.Employee;
import org.springframework.stereotype.Service;

import java.io.Serializable;
import java.util.List;


public interface EmployeeService extends IService<Employee> {
    // 根据 ID 查询单个员工
    Employee getEmployeeById(Long id);

    // 查询所有员工
    List<Employee> getAllEmployees();
    List<Employee> selectAllEmployees();
    // 根据条件查询员工
    List<Employee> getEmployeesByPosition(String position);
    List<Employee> getEmployeesByPage(int PageNo, int PageSize);
    List<Employee> selectdor();
    // 插入员工
    int insertEmployee(Employee employee);

    void addateEmployee(Employee employee);

    // 根据 ID 删除员工
    int deleteEmployeeById(Serializable id);

    // 更新员工信息
    int updateEmployee(Employee employee);

    // 更新所有员工
    int updateAllEmployee(Employee updateEntity, String firstname);


    // 删除所有员工
    int deleteAllEmployees();

    IPage<Employee> getEmployeeByPage(int pageNo, int pageSize);

}